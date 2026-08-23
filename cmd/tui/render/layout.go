// SPDX-FileContributor: FlameFlag <github@flameflag.dev>
// SPDX-FileContributor: 4evy <git@evy.pink>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package render

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/asciimoo/hister/cmd/tui/component"
	"github.com/asciimoo/hister/cmd/tui/model"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

type overlayDef struct {
	content func(*model.Model) string
	border  func(*model.Model) color.Color
	offset  func(*model.Model) (int, int) // nil = use OverlayOff
}

var overlayDefs = map[model.ViewState]overlayDef{
	model.StateHelp:            {HelpOverlay, func(m *model.Model) color.Color { return m.Styles.HelpBorder }, nil},
	model.StateDialog:          {func(m *model.Model) string { return DeleteDialog(m) }, func(m *model.Model) color.Color { return m.Styles.DialogBorder }, nil},
	model.StateThemePicker:     {func(m *model.Model) string { return ThemePicker(m) }, func(m *model.Model) color.Color { return m.Styles.ThemeBorder }, nil},
	model.StateSettings:        {func(m *model.Model) string { return Settings(m) }, func(m *model.Model) color.Color { return m.Styles.HelpBorder }, nil},
	model.StateContextMenu:     {func(m *model.Model) string { return ContextMenu(m) }, func(m *model.Model) color.Color { return m.Styles.DialogBorder }, MenuOverlayOffset},
	model.StatePrioritizeInput: {func(m *model.Model) string { return PrioritizeInput(m) }, func(m *model.Model) color.Color { return m.Styles.DialogBorder }, nil},
	model.StateLabelInput:      {LabelInput, func(m *model.Model) color.Color { return m.Styles.ThemeBorder }, nil},
}

func View(m *model.Model) string {
	if !m.Ready {
		return "Loading..."
	}
	if m.Width < 20 || m.Height < 14 {
		return "Terminal too small"
	}

	main := MainView(m)

	if def, ok := overlayDefs[m.State]; ok {
		maxW := overlayMaxWidth(m)
		fg := renderOverlayBox(def.content(m), def.border(m), maxW)
		offX, offY := m.OverlayOffX, m.OverlayOffY
		if def.offset != nil {
			offX, offY = def.offset(m)
		}
		return renderOverlay(main, fg, m.Width-1, m.Height, offX, offY)
	}
	return main
}

var tabRenderers = map[int]func(*model.Model) string{
	model.TabHistory: HistoryTab,
	model.TabRules:   RulesTab,
	model.TabAdd:     AddTab,
}

func MainView(m *model.Model) string {
	w := max(0, m.Width-1)
	div := m.Styles.Div.Render(strings.Repeat("─", w))

	header := Header(m)

	if renderer, ok := tabRenderers[m.ActiveTab]; ok {
		content := renderer(m)
		m.Workspace.SetContent(content)
		target := m.WorkspaceSelectionY
		if target < m.Workspace.YOffset() {
			m.Workspace.SetYOffset(target)
		} else if target >= m.Workspace.YOffset()+m.Workspace.Height() {
			m.Workspace.SetYOffset(max(0, target-m.Workspace.Height()+2))
		}
		hints := Hints(m)
		return strings.Join([]string{header, div, m.Workspace.View(), div, hints}, "\n")
	}

	pStyle := m.Styles.PromptActive
	if m.State != model.StateInput {
		pStyle = m.Styles.PromptBlur
	}
	inputLine := "  " + pStyle.Render("❯") + " " + m.TextInput.View()
	ResizeSearchViewports(m)
	body := SearchBody(m)

	hints := Hints(m)

	return strings.Join([]string{header, div, inputLine, div, body, div, hints}, "\n")
}

// DetailsVisible reports whether the Search workspace has a readable preview
// open. It is intentionally independent of State so a label/dialog overlay can
// retain the split pane behind it.
func DetailsVisible(m *model.Model) bool {
	return m.ActiveTab == model.TabSearch && m.DetailsURL != ""
}

func DetailsSplit(m *model.Model) bool {
	return DetailsVisible(m) && m.Width >= model.DetailsSplitMinWidth
}

func DetailsPaneWidth(m *model.Model) int {
	w := max(1, m.Width-1)
	if !DetailsSplit(m) {
		return w
	}
	return min(model.DetailsPaneMaxWidth, max(model.DetailsPaneMinWidth, w*46/100))
}

// ResizeSearchViewports applies the same geometry that SearchBody renders.
// Keeping this calculation in one place prevents wrapping, scrollbar, and
// pointer hitboxes from disagreeing when the details pane opens or closes.
func ResizeSearchViewports(m *model.Model) {
	w := max(1, m.Width-1)
	bodyH := max(1, m.Height-model.FixedLayoutRows)
	leftW := w
	if DetailsSplit(m) {
		leftW -= DetailsPaneWidth(m)
	}
	m.Viewport.SetWidth(max(1, leftW-2))
	m.Viewport.SetHeight(bodyH)

	if DetailsVisible(m) {
		m.Details.SetWidth(max(1, DetailsPaneWidth(m)-2))
		m.Details.SetHeight(max(1, bodyH-2))
	}
}

// DetailsPaneBounds returns the screen-space pane body used by mouse hit
// testing. Y begins below the search input and its divider.
func DetailsPaneBounds(m *model.Model) (x, y, width, height int) {
	width = DetailsPaneWidth(m)
	x = 0
	if DetailsSplit(m) {
		x = max(0, m.Width-1-width)
	}
	return x, model.RowVPStart, width, max(1, m.Height-model.FixedLayoutRows)
}

func SearchBody(m *model.Model) string {
	w := max(1, m.Width-1)
	bodyH := max(1, m.Height-model.FixedLayoutRows)
	if DetailsVisible(m) && !DetailsSplit(m) {
		return DetailsPane(m, w, bodyH)
	}

	leftW := w
	if DetailsSplit(m) {
		leftW -= DetailsPaneWidth(m)
	}
	results := resultsViewport(m, leftW, bodyH)
	if !DetailsSplit(m) {
		return results
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, results, DetailsPane(m, DetailsPaneWidth(m), bodyH))
}

func resultsViewport(m *model.Model, width, height int) string {
	content := normalizeBlock(m.Viewport.View(), max(1, m.Viewport.Width()), height)
	if m.Viewport.TotalLineCount() > m.Viewport.Height() && m.Viewport.Height() > 0 {
		content = lipgloss.JoinHorizontal(lipgloss.Top, content, " ", Scrollbar(m))
	}
	return normalizeBlock(content, width, height)
}

func DetailsPane(m *model.Model, width, height int) string {
	innerW := max(1, width-2)
	headerStyle := m.Styles.Gray
	if m.DetailsFocused || !DetailsSplit(m) {
		headerStyle = m.Styles.HelpHeader
	}
	title := headerStyle.Render("Preview")
	if m.DetailsLoading {
		title += " " + m.Styles.Spin.Render(m.Spinner.View())
	}
	closeButton := m.Styles.HintKey.Render("×")
	header := truncateAnsi(title, max(1, innerW-2))
	header = rightPad(header, max(1, innerW-lipgloss.Width(closeButton))) + closeButton
	divider := m.Styles.Div.Render(strings.Repeat("─", innerW))
	body := normalizeBlock(m.Details.View(), innerW, max(1, height-2))
	content := strings.Join([]string{header, divider, body}, "\n")
	content = normalizeBlock(content, innerW, height)

	prefix := m.Styles.Div.Render("│") + " "
	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func normalizeBlock(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		line = truncateAnsi(line, width)
		lines[i] = rightPad(line, width)
	}
	return strings.Join(lines, "\n")
}

func Header(m *model.Model) string {
	w := max(1, m.Width-1)
	compact := w < 56
	var tabs []string
	m.TabTargets = nil
	x := model.TabBarLeftPad
	for _, definition := range model.Tabs {
		label := definition.Name
		if compact {
			label = definition.Name[:1]
		}
		var tab string
		if definition.ID == m.ActiveTab {
			tab = m.Styles.TabActive.Render("[" + label + "]")
		} else {
			tab = m.Styles.TabInactive.Render(" " + label + " ")
		}
		tabs = append(tabs, tab)
		tabWidth := lipgloss.Width(tab)
		m.TabTargets = append(m.TabTargets, model.HintRegion{
			X0: x, X1: x + tabWidth, Action: definition.Action,
		})
		x += tabWidth + model.TabGap
	}
	tabBar := " " + strings.Join(tabs, " ")
	appendMode := func(full, short string) {
		label := full
		if compact {
			label = short
		}
		badge := "  " + m.Styles.Conn.Render(label)
		if lipgloss.Width(tabBar)+lipgloss.Width(badge) < w {
			tabBar += badge
		}
	}
	if m.SortMode == "domain" {
		appendMode("[domain]", "D")
	}
	if m.SemanticOn {
		appendMode("[semantic]", "S")
	}

	cs := m.Styles.Disc.Render("● disconnected")
	if m.WsReady {
		cs = m.Styles.Conn.Render("● connected")
	}

	var right string
	if m.Notice != "" {
		right = m.Styles.Conn.Render("◆ " + m.Notice)
	} else if m.ConnError != nil && !m.WsReady {
		right = m.Styles.Disc.Render(m.ConnError.Error())
	} else if m.IsSearching {
		right = cs + "  " + m.Styles.Spin.Render(m.Spinner.View()+" searching…")
	} else {
		countStr := "0 results"
		if m.Results != nil {
			visibleCount := len(m.Results.History) + len(m.VisibleDocuments())
			countStr = fmt.Sprintf("%d results", max(int(m.Results.Total)+len(m.Results.History), visibleCount))
			if m.Results.SearchDuration != "" {
				countStr += "  " + m.Results.SearchDuration
			}
		}
		right = cs + "  " + m.Styles.Status.Render(countStr)
	}

	leftW := lipgloss.Width(tabBar)
	rightW := lipgloss.Width(right)
	if available := max(0, w-leftW); rightW > available {
		right = truncateAnsi(right, available)
		rightW = lipgloss.Width(right)
	}
	pad := max(0, w-leftW-rightW)
	return tabBar + strings.Repeat(" ", pad) + right
}

func keyContext(m *model.Model) component.KeyContext {
	if m.State == model.StateDetails {
		return component.ContextDetails
	}
	switch m.ActiveTab {
	case model.TabHistory:
		return component.ContextHistory
	case model.TabRules:
		return component.ContextRules
	case model.TabAdd:
		return component.ContextAdd
	default:
		return component.ContextSearch
	}
}

func Hints(m *model.Model) string {
	h := m.Help
	h.ShowAll = false
	h.SetWidth(max(1, m.Width-4))
	if m.ThemeName == "no-color" {
		h.ShortSeparator = " | "
	} else {
		h.ShortSeparator = " · "
	}
	return "  " + h.View(m.Keys.For(keyContext(m)))
}

func HelpOverlay(m *model.Model) string {
	h := m.Help
	h.ShowAll = true
	h.SetWidth(max(20, overlayMaxWidth(m)-8))
	content := m.Styles.HelpHeader.Render("Keyboard shortcuts") + "\n\n" +
		h.View(m.Keys.For(keyContext(m)))
	return m.Styles.Help.Render(content)
}

func overlayMaxWidth(m *model.Model) int {
	return max(20, m.Width-5)
}

// wraps content in a rounded border with drag handle and close button.
func renderOverlayBox(content string, borderColor color.Color, maxWidth int) string {
	lines := strings.Split(content, "\n")
	maxW := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > maxW {
			maxW = w
		}
	}
	if maxWidth > 0 && maxW > maxWidth {
		maxW = maxWidth
		for i, l := range lines {
			if lipgloss.Width(l) > maxW {
				lines[i] = truncateAnsi(l, maxW)
			}
		}
	}

	bc := lipgloss.NewStyle().Foreground(borderColor)
	closeSt := lipgloss.NewStyle().Foreground(borderColor).Bold(true)

	handle := " ≡ "
	closeBtn := closeSt.Render("[x]")
	closeBtnW := lipgloss.Width(closeBtn)
	handleW := len([]rune(handle))
	barW := maxW
	leftHandleW := (barW - handleW) / 2
	rightHandleW := max(barW-handleW-leftHandleW-closeBtnW, 0)
	topBar := bc.Render("╭"+strings.Repeat("─", leftHandleW)+handle+strings.Repeat("─", rightHandleW)) + closeBtn + bc.Render("╮")
	bottomBar := bc.Render("╰" + strings.Repeat("─", maxW) + "╯")

	var sb strings.Builder
	sb.WriteString(topBar)
	for _, l := range lines {
		sb.WriteByte('\n')
		pad := max(0, maxW-lipgloss.Width(l))
		sb.WriteString(bc.Render("│"))
		sb.WriteString(l)
		sb.WriteString(strings.Repeat(" ", pad))
		sb.WriteString(bc.Render("│"))
	}
	sb.WriteByte('\n')
	sb.WriteString(bottomBar)
	return sb.String()
}

// dimCanvas applies faint styling to the already-parsed background cells.
// Working at the Charm cell layer preserves nested colors and attributes
// without hand-editing CSI resets in the rendered string.
func dimCanvas(canvas *lipgloss.Canvas) {
	for y := range canvas.Height() {
		for x := range canvas.Width() {
			cell := canvas.CellAt(x, y)
			if cell == nil {
				continue
			}
			cell = cell.Clone()
			cell.Style.Attrs |= uv.AttrFaint
			canvas.SetCell(x, y, cell)
		}
	}
}

func renderOverlay(bg, fg string, bgW, bgH, offX, offY int) string {
	fgW, fgH := lipgloss.Width(fg), lipgloss.Height(fg)
	startY := max(0, min(bgH-fgH, (bgH-fgH)/2+offY))
	startX := max(0, min(bgW-fgW, (bgW-fgW)/2+offX))

	canvas := lipgloss.NewCanvas(bgW, bgH)
	canvas.Compose(lipgloss.NewCompositor(lipgloss.NewLayer(bg)))
	dimCanvas(canvas)
	canvas.Compose(lipgloss.NewCompositor(lipgloss.NewLayer(fg).X(startX).Y(startY)))
	return canvas.Render()
}

func MenuOverlayOffset(m *model.Model) (int, int) {
	fg := renderOverlayBox(ContextMenu(m), m.Styles.DialogBorder, overlayMaxWidth(m))
	fgLines := strings.Split(fg, "\n")
	fgH := len(fgLines)
	fgW := 0
	for _, l := range fgLines {
		if w := lipgloss.Width(l); w > fgW {
			fgW = w
		}
	}
	bgW, bgH := m.Width-1, m.Height
	offX := m.MenuX - (bgW-fgW)/2
	offY := m.MenuY - (bgH-fgH)/2
	return offX, offY
}

func OverlayBounds(m *model.Model) (x, y, w, h int) {
	def, ok := overlayDefs[m.State]
	if !ok {
		return
	}
	fg := renderOverlayBox(def.content(m), def.border(m), overlayMaxWidth(m))
	fgLines := strings.Split(fg, "\n")
	h = len(fgLines)
	for _, l := range fgLines {
		if lw := lipgloss.Width(l); lw > w {
			w = lw
		}
	}
	bgW, bgH := m.Width-1, m.Height
	y = max(0, min(bgH-h, (bgH-h)/2+m.OverlayOffY))
	x = max(0, min(bgW-w, (bgW-w)/2+m.OverlayOffX))
	return
}

func ComputeHintRegions(m *model.Model) []model.HintRegion {
	keyMap := m.Keys.For(keyContext(m))
	hints := keyMap.ShortHints()
	var separator string
	if m.ThemeName == "no-color" {
		separator = " | "
	} else {
		separator = " · "
	}
	sepW := lipgloss.Width(m.Help.Styles.ShortSeparator.Render(separator))

	var regions []model.HintRegion
	x := 2
	for i, hint := range hints {
		if i > 0 {
			x += sepW
		}
		help := hint.Binding.Help()
		hintW := lipgloss.Width(m.Help.Styles.ShortKey.Render(help.Key)) + 1 +
			lipgloss.Width(m.Help.Styles.ShortDesc.Render(help.Desc))
		regions = append(regions, model.HintRegion{X0: x, X1: x + hintW, Action: hint.Action})
		x += hintW
	}
	return regions
}
