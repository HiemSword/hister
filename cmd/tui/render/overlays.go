// SPDX-FileContributor: 4evy <git@evy.pink>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package render

import (
	"fmt"
	"strings"

	"github.com/asciimoo/hister/cmd/tui/model"
	"github.com/asciimoo/hister/cmd/tui/theme"
	"github.com/asciimoo/hister/config"

	"charm.land/lipgloss/v2"
)

func ThemePicker(m *model.Model) string {
	darkNames, lightNames := theme.ClassifyThemes()
	listBudget := max(2, m.Height-12)
	darkBudget := (listBudget + 1) / 2
	lightBudget := listBudget - darkBudget
	darkStart, darkEnd := windowRange(len(darkNames), m.DarkThemeIdx, darkBudget)
	lightStart, lightEnd := windowRange(len(lightNames), m.LightThemeIdx, lightBudget)
	m.ThemeDarkStart, m.ThemeDarkCount = darkStart, darkEnd-darkStart
	m.ThemeLightStart, m.ThemeLightCount = lightStart, lightEnd-lightStart

	maxNameW := 0
	for _, name := range append(darkNames, lightNames...) {
		if len(name) > maxNameW {
			maxNameW = len(name)
		}
	}

	modes := []string{"auto", "dark", "light"}
	var modeParts []string
	for _, mode := range modes {
		if mode == m.ThemePickerMode {
			modeParts = append(modeParts, m.Styles.ThemePickerSelected.Render("["+mode+"]"))
		} else {
			modeParts = append(modeParts, m.Styles.Gray.Render(" "+mode+" "))
		}
	}
	modeRow := "Mode: " + strings.Join(modeParts, " ")

	renderSection := func(names []string, start, end, cursorIdx int, configuredName string, focused bool) []string {
		var slines []string
		for i, name := range names[start:end] {
			absoluteIdx := start + i
			marker := "  "
			if name == configuredName {
				marker = "● "
			}
			paddedName := name + strings.Repeat(" ", maxNameW-len(name))
			swatch := ""
			if p, ok := theme.GetPalette(name); ok {
				swatch = renderSwatch(p)
			}
			content := marker + paddedName + "  " + swatch
			if focused && absoluteIdx == cursorIdx {
				slines = append(slines, m.Styles.ThemePickerSelected.Render(content))
			} else {
				slines = append(slines, m.Styles.ThemePickerItem.Render(content))
			}
		}
		return slines
	}

	var lines []string
	lines = append(lines, modeRow, "")

	darkFocused := m.ThemePickerSection == 0
	headerStyle := m.Styles.Gray
	if darkFocused {
		headerStyle = m.Styles.SelTitle
	}
	lines = append(lines, headerStyle.Render(rangeHeader("Dark Themes", darkStart, darkEnd, len(darkNames))))
	lines = append(lines, renderSection(darkNames, darkStart, darkEnd, m.DarkThemeIdx, m.Cfg.TUI.DarkTheme, darkFocused)...)
	lines = append(lines, "")

	lightFocused := m.ThemePickerSection == 1
	headerStyle = m.Styles.Gray
	if lightFocused {
		headerStyle = m.Styles.SelTitle
	}
	lines = append(lines, headerStyle.Render(rangeHeader("Light Themes", lightStart, lightEnd, len(lightNames))))
	lines = append(lines, renderSection(lightNames, lightStart, lightEnd, m.LightThemeIdx, m.Cfg.TUI.LightTheme, lightFocused)...)

	lines = append(lines, "")
	nav := m.Keys.BestKey(config.ActionScrollDown)
	mode := m.Keys.BestKey(config.ActionToggleTheme)
	confirm := m.Keys.BestKey(config.ActionOpenResult)
	themeHints := nav + " navigate  ⇥ section  " + mode + " mode  " + confirm + " confirm  ⎋ cancel"
	lines = append(lines, m.Styles.Hint.Render(themeHints))
	return m.Styles.ThemePicker.Render(strings.Join(lines, "\n"))
}

// ContextMenu renders a small context menu box.
func ContextMenu(m *model.Model) string {
	var lines []string
	for i, option := range model.MenuOptions {
		if i == m.MenuSelIdx {
			lines = append(lines, m.Styles.ThemePickerSelected.Render("▸ "+option.Label))
		} else {
			lines = append(lines, m.Styles.ThemePickerItem.Render("  "+option.Label))
		}
	}
	return m.Styles.Dialog.Render(strings.Join(lines, "\n"))
}

func DeleteDialog(m *model.Model) string {
	var lines []string
	lines = append(lines, m.Styles.Title.Render(m.DialogMsg))
	lines = append(lines, "")
	urlDisplay := truncateLine(m.DialogURL, 35)
	lines = append(lines, m.Styles.URL.Render(urlDisplay))
	lines = append(lines, "")
	cancelLabel := " Cancel "
	deleteLabel := " Delete "
	var cancelBtn, deleteBtn string
	if m.DialogBtnIdx == 0 {
		cancelBtn = m.Styles.CancelBtnSel.Render(cancelLabel)
	} else {
		cancelBtn = m.Styles.CancelBtn.Render(cancelLabel)
	}
	if m.DialogBtnIdx == 1 {
		deleteBtn = m.Styles.DeleteBtnSel.Render(deleteLabel)
	} else {
		deleteBtn = m.Styles.DeleteBtn.Render(deleteLabel)
	}
	lines = append(lines, cancelBtn+"   "+deleteBtn)
	lines = append(lines, "")
	lines = append(lines, m.Styles.Hint.Render("←/→ select  ↵ confirm  esc cancel"))
	return m.Styles.Dialog.Render(strings.Join(lines, "\n"))
}

func Settings(m *model.Model) string {
	items := m.SortedSettingsItems()
	errorRows := 0
	if m.SettingsEditErr != "" {
		errorRows = 2
	}
	start, end := windowRange(len(items), m.SettingsIdx, max(1, m.Height-8-errorRows))

	maxKeyW := 0
	for _, it := range items {
		fk := FormatKey(it.Key)
		if len([]rune(fk)) > maxKeyW {
			maxKeyW = len([]rune(fk))
		}
	}

	var lines []string
	lines = append(lines, m.Styles.Title.Render(rangeHeader("Keybindings", start, end, len(items))))
	lines = append(lines, "")
	for i, it := range items[start:end] {
		absoluteIdx := start + i
		if absoluteIdx == m.SettingsIdx && m.SettingsEditMode {
			lines = append(lines, m.Styles.ThemePickerSelected.Render("  Press a key...  →  "+string(it.Action)))
		} else {
			fk := FormatKey(it.Key)
			padded := fk + strings.Repeat(" ", maxKeyW-len([]rune(fk)))
			row := "  " + padded + "  →  " + string(it.Action)
			if absoluteIdx == m.SettingsIdx {
				lines = append(lines, m.Styles.ThemePickerSelected.Render(row))
			} else {
				lines = append(lines, m.Styles.ThemePickerItem.Render(row))
			}
		}
	}
	if m.SettingsEditErr != "" {
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().Foreground(m.Styles.DialogBorder).Render("  "+m.SettingsEditErr))
	}
	lines = append(lines, "")
	if m.SettingsEditMode {
		lines = append(lines, m.Styles.Hint.Render("press any key to bind  esc cancel"))
	} else {
		sNav := m.Keys.BestKey(config.ActionScrollDown)
		sEdit := m.Keys.BestKey(config.ActionOpenResult)
		lines = append(lines, m.Styles.Hint.Render(sNav+" navigate  "+sEdit+" edit  ⎋ close"))
	}
	return m.Styles.Help.Render(strings.Join(lines, "\n"))
}

func windowRange(length, cursor, budget int) (int, int) {
	if length <= 0 || budget <= 0 {
		return 0, 0
	}
	budget = min(length, budget)
	start := max(0, min(cursor-budget/2, length-budget))
	return start, start + budget
}

func rangeHeader(label string, start, end, total int) string {
	if start == 0 && end == total {
		return label
	}
	return fmt.Sprintf("%s (%d–%d/%d)", label, start+1, end, total)
}

func PrioritizeInput(m *model.Model) string {
	var lines []string
	lines = append(lines, m.Styles.Title.Render("Add Priority Pattern"))
	lines = append(lines, "")
	lines = append(lines, m.Styles.Gray.Render("Pattern:"))
	lines = append(lines, "  "+m.PrioritizeInput.View())
	lines = append(lines, "")
	cancelLabel := " Cancel "
	confirmLabel := " Confirm "
	var cancelBtn, confirmBtn string
	if m.PrioritizeBtnIdx == 0 {
		cancelBtn = m.Styles.CancelBtnSel.Render(cancelLabel)
	} else {
		cancelBtn = m.Styles.CancelBtn.Render(cancelLabel)
	}
	if m.PrioritizeBtnIdx == 1 {
		confirmBtn = m.Styles.ConfirmBtnSel.Render(confirmLabel)
	} else {
		confirmBtn = m.Styles.ConfirmBtn.Render(confirmLabel)
	}
	lines = append(lines, cancelBtn+"   "+confirmBtn)
	lines = append(lines, "")
	lines = append(lines, m.Styles.Hint.Render("←/→ select  ↵ confirm  esc cancel"))
	return m.Styles.Dialog.Render(strings.Join(lines, "\n"))
}

func LabelInput(m *model.Model) string {
	lines := []string{
		m.Styles.Title.Render("Edit label"),
		"",
		m.Styles.URL.Render(truncateLine(m.LabelURL, max(20, m.LabelInput.Width()))),
		"",
		m.LabelInput.View(),
		"",
		m.Styles.Hint.Render("↵ save  ⎋ cancel  empty label clears it"),
	}
	return m.Styles.Dialog.Render(strings.Join(lines, "\n"))
}

func renderSwatch(p theme.Palette) string {
	colors := []string{p.Base01, p.Base08, p.Base09, p.Base0A, p.Base0B, p.Base0C, p.Base0D, p.Base0E}
	var sb strings.Builder
	for _, hex := range colors {
		sb.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(hex)).Render("  "))
	}
	return sb.String()
}

func RefreshViewport(m *model.Model) {
	if m.Ready {
		m.Viewport.SetContent(Results(m))
	}
}

func RefreshAndScroll(m *model.Model) {
	RefreshViewport(m)
	m.ScrollToSelected()
}
