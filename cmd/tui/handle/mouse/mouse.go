// SPDX-FileContributor: FlameFlag <github@flameflag.dev>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package mouse

import (
	"github.com/asciimoo/hister/cmd/tui/model"
	"github.com/asciimoo/hister/cmd/tui/render"
	"github.com/asciimoo/hister/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/browser"
	"github.com/rs/zerolog/log"
)

type Deps struct {
	ExecuteAction              func(*model.Model, config.Action) tea.Cmd
	SwitchTab                  func(*model.Model, config.Action) tea.Cmd
	StartSearch                func(*model.Model, ...tea.Cmd) tea.Cmd
	CloseOverlay               func(*model.Model) tea.Cmd
	SubmitAdd                  func(*model.Model) tea.Cmd
	CloseThemePickerWithRevert func(*model.Model) tea.Cmd
	PreviewTheme               func(*model.Model)
	ExecuteContextMenuAction   func(*model.Model) tea.Cmd
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{d} }

type action uint8

const (
	actionClick action = iota
	actionRelease
	actionMotion
	actionWheel
)

// Event normalizes mouse messages for the layout hit testing shared by tabs,
// overlays, and the results viewport.
type Event struct {
	X, Y   int
	Button tea.MouseButton
	Action action
}

func newEvent(msg tea.MouseMsg) Event {
	e := Event{X: msg.X, Y: msg.Y, Button: msg.Button}
	switch {
	case msg.Action == tea.MouseActionRelease:
		e.Action = actionRelease
	case msg.Action == tea.MouseActionMotion:
		e.Action = actionMotion
	case tea.MouseEvent(msg).IsWheel():
		e.Action = actionWheel
	default:
		e.Action = actionClick
	}
	return e
}

type Region struct{ X, Y, W, H int }

func (r Region) Contains(msg Event) bool {
	return msg.X >= r.X && msg.X < r.X+r.W && msg.Y >= r.Y && msg.Y < r.Y+r.H
}

func (r Region) ContainsY(y int) bool {
	return y >= r.Y && y < r.Y+r.H
}

// --- helpers ---

func vpRegion(m *model.Model) Region {
	top := model.RowVPStart
	bottom := model.RowVPEnd(m.Height)
	return Region{X: 0, Y: top, W: m.Width, H: bottom - top + 1}
}

func scrollToPercent(m *model.Model, mouseY int) {
	vp := vpRegion(m)
	if vp.H <= 1 {
		return
	}
	maxScroll := m.TotalLines - m.Viewport.Height
	if maxScroll <= 0 {
		return
	}
	relY := max(0, min(mouseY-vp.Y, vp.H-1))
	pct := float64(relY) / float64(vp.H-1)
	m.Viewport.SetYOffset(int(pct * float64(maxScroll)))
	contentY := m.Viewport.YOffset + m.Viewport.Height/2
	if idx := m.FindResultAtY(contentY); idx >= 0 {
		m.SelectedIdx = idx
	}
	render.RefreshViewport(m)
}

func wheelDelta(msg Event) int {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return -1
	case tea.MouseButtonWheelDown:
		return 1
	default:
		return 0
	}
}

func isLeftClick(msg Event) bool {
	return msg.Action == actionClick && msg.Button == tea.MouseButtonLeft
}

func isOverlayState(s model.ViewState) bool {
	switch s {
	case model.StateHelp, model.StateDialog, model.StateThemePicker,
		model.StateSettings, model.StateContextMenu, model.StatePrioritizeInput:
		return true
	default:
		return false
	}
}

// handleScroll applies a wheel event to idx (clamped to [lo, hi]) and calls
// after when the index changes. Returns (nil, true) if a wheel event was
// consumed, (nil, false) otherwise.
func handleScroll(msg Event, idx *int, lo, hi int, after func()) (tea.Cmd, bool) {
	delta := wheelDelta(msg)
	if delta == 0 {
		return nil, false
	}
	if lo <= hi {
		if model.ScrollIdx(idx, delta, lo, hi) && after != nil {
			after()
		}
	}
	return nil, true
}

// --- Handler methods ---

func (h *Handler) closeOverlayForState(m *model.Model) tea.Cmd {
	if m.State == model.StateThemePicker {
		return h.CloseThemePickerWithRevert(m)
	}
	return h.CloseOverlay(m)
}

func (h *Handler) hintRegions(m *model.Model, msg Event) tea.Cmd {
	regions := render.ComputeHintRegions(m)
	for _, r := range regions {
		if msg.X >= r.X0 && msg.X < r.X1 {
			return h.ExecuteAction(m, r.Action)
		}
	}
	return nil
}

// Handle is the main entry point for mouse events.
func (h *Handler) Handle(m *model.Model, msg tea.MouseMsg) tea.Cmd {
	event := newEvent(msg)
	if m.ScrollbarDragging {
		if event.Action == actionMotion {
			scrollToPercent(m, event.Y)
			return nil
		}
		if event.Action == actionRelease {
			m.ScrollbarDragging = false
			return nil
		}
	}

	if isOverlayState(m.State) {
		return h.overlay(m, event)
	}
	if m.ActiveTab != model.TabSearch {
		return h.nonSearchTab(m, event)
	}

	if cmd, ok := handleScroll(event, &m.SelectedIdx, 0, m.GetTotalResults()-1, func() {
		render.RefreshAndScroll(m)
	}); ok {
		return cmd
	}

	if event.Action == actionClick && event.Button == tea.MouseButtonRight {
		return rightClick(m, event)
	}

	if !isLeftClick(event) {
		return nil
	}

	if event.Y == model.RowTabBar {
		return h.tabBar(m, event)
	}
	if event.Y == model.RowInput {
		return inputRow(m, event)
	}
	if event.Y == model.RowHints(m.Height) {
		return h.hintRegions(m, event)
	}
	if m.TotalLines > m.Viewport.Height && m.Viewport.Height > 0 && event.X >= m.Width-model.ScrollbarWidth {
		return scrollbarClick(m, event)
	}
	return h.viewportClick(m, event)
}

// --- search-tab handlers ---

func rightClick(m *model.Model, msg Event) tea.Cmd {
	vp := vpRegion(m)
	if !vp.ContainsY(msg.Y) || len(m.LineOffsets) == 0 {
		return nil
	}
	contentY := (msg.Y - vp.Y) + m.Viewport.YOffset
	idx := m.FindResultAtY(contentY)
	if idx < 0 || idx >= m.GetTotalResults() || idx == m.Limit {
		return nil
	}
	m.SelectedIdx = idx
	render.RefreshViewport(m)
	offX, offY := render.MenuOverlayOffset(m)
	m.OpenContextMenu(idx, msg.X, msg.Y, offX, offY)
	return nil
}

func inputRow(m *model.Model, msg Event) tea.Cmd {
	m.State = model.StateInput
	prefixW := model.InputLeadingPad + lipgloss.Width("❯") + model.InputTrailingPad
	pos := min(max(msg.X-prefixW, 0), len([]rune(m.TextInput.Value())))
	m.TextInput.SetCursor(pos)
	return m.TextInput.Focus()
}

func scrollbarClick(m *model.Model, msg Event) tea.Cmd {
	vp := vpRegion(m)
	if vp.ContainsY(msg.Y) {
		m.ScrollbarDragging = true
		scrollToPercent(m, msg.Y)
	}
	return nil
}

func (h *Handler) viewportClick(m *model.Model, msg Event) tea.Cmd {
	vp := vpRegion(m)
	if !vp.ContainsY(msg.Y) || len(m.LineOffsets) == 0 {
		return nil
	}
	contentY := (msg.Y - vp.Y) + m.Viewport.YOffset
	if m.SuggestionHeight > 0 && contentY < m.SuggestionHeight && m.Results != nil && m.Results.QuerySuggestion != "" {
		m.TextInput.SetValue(m.Results.QuerySuggestion)
		m.TextInput.SetCursor(len([]rune(m.Results.QuerySuggestion)))
		m.SelectedIdx = -1
		m.Limit = model.ResultsPageSize
		return h.StartSearch(m)
	}
	idx := m.FindResultAtY(contentY)
	if idx < 0 || idx >= m.GetTotalResults() {
		return nil
	}
	if m.State == model.StateInput {
		m.State = model.StateResults
		m.TextInput.Blur()
	}
	if idx == m.SelectedIdx {
		if m.SelectedIdx == m.Limit {
			m.Limit += model.ResultsPageSize
			render.RefreshAndScroll(m)
			return h.StartSearch(m)
		} else if u := m.GetSelectedURL(); u != "" {
			if err := browser.OpenURL(u); err != nil {
				log.Warn().Err(err).Msg("failed to open URL in browser")
			}
			return m.PostHistoryCmd(u)
		}
	} else {
		m.SelectedIdx = idx
		render.RefreshAndScroll(m)
	}
	return nil
}

// --- shared ---

func (h *Handler) tabBar(m *model.Model, msg Event) tea.Cmd {
	for _, target := range m.TabTargets {
		if msg.X >= target.X0 && msg.X < target.X1 {
			return h.SwitchTab(m, target.Action)
		}
	}
	return nil
}
