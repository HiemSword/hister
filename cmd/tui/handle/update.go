// SPDX-FileContributor: FlameFlag <github@flameflag.dev>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package handle

import (
	"time"

	"github.com/asciimoo/hister/cmd/tui/handle/mouse"
	"github.com/asciimoo/hister/cmd/tui/model"
	"github.com/asciimoo/hister/cmd/tui/network"
	"github.com/asciimoo/hister/cmd/tui/render"
	"github.com/asciimoo/hister/config"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

var mouseHandler = mouse.New(mouse.Deps{
	ExecuteAction:              ExecuteAction,
	SwitchTab:                  SwitchTab,
	StartSearch:                startSearch,
	CloseOverlay:               CloseOverlay,
	SubmitAdd:                  submitAdd,
	CloseThemePickerWithRevert: CloseThemePickerWithRevert,
	PreviewTheme:               previewTheme,
	ExecuteContextMenuAction:   executeContextMenuAction,
})

func Update(m *model.Model, msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		changed := m.Width != msg.Width || m.Height != msg.Height
		m.Width, m.Height = msg.Width, msg.Height

		vpH := max(0, m.Height-model.FixedLayoutRows)
		m.TextInput.Width = max(1, m.Width-6)
		formW := max(12, min(72, m.Width-12))
		for i := range m.AddInputs {
			m.AddInputs[i].Width = formW
		}
		m.AddText.SetWidth(formW)
		for i := range m.RulesPatternInputs {
			m.RulesPatternInputs[i].Width = max(12, min(48, m.Width-24))
		}
		m.Workspace.Width = max(1, m.Width-1)
		m.Workspace.Height = max(1, vpH+2)
		m.Help.Width = max(1, m.Width-4)

		if !m.Ready {
			m.Viewport = viewport.New(1, vpH)
			m.Viewport.SetContent("")
			m.Ready = true
			render.ResizeSearchViewports(m)
			return tea.ClearScreen
		}
		render.ResizeSearchViewports(m)
		render.RefreshAndScroll(m)
		if changed {
			return tea.ClearScreen
		}
		return nil

	case tea.KeyMsg:
		if m.Keys.Action(msg) == config.ActionQuit {
			return tea.Quit
		}
		switch m.State {
		case model.StateDialog:
			return DialogKeys(m, msg)
		case model.StateInput:
			if m.ActiveTab != model.TabSearch {
				return TabKeys(m, msg)
			}
			return InputKeys(m, msg)
		case model.StateResults:
			if m.ActiveTab != model.TabSearch {
				return TabKeys(m, msg)
			}
			return ResultsKeys(m, msg)
		case model.StateHelp:
			m.State = m.PrevState
			if m.State == model.StateInput {
				return m.TextInput.Focus()
			}
			return nil
		case model.StateThemePicker:
			return ThemePickerKeys(m, msg)
		case model.StateContextMenu:
			return ContextMenuKeys(m, msg)
		case model.StateSettings:
			return SettingsKeys(m, msg)
		case model.StatePrioritizeInput:
			return PrioritizeInputKeys(m, msg)
		}

	case tea.MouseMsg:
		return mouseHandler.Handle(m, msg)

	case spinner.TickMsg:
		if m.IsSearching || m.HistoryLoading || m.RulesLoading {
			var cmd tea.Cmd
			m.Spinner, cmd = m.Spinner.Update(msg)
			return cmd
		}

	case model.HintClearMsg:
		m.HintFlash = ""
	case model.SettingsErrClearMsg:
		m.SettingsEditErr = ""
	case model.NoticeClearMsg:
		if msg.ID == m.NoticeID {
			m.Notice = ""
		}
	case model.HistoryFetchedMsg:
		m.HistoryLoading = false
		if msg.Err != nil {
			return m.Notify("Could not load history: " + msg.Err.Error())
		}
		m.HistoryItems = msg.Items
		m.HistoryIdx = 0

	case model.RulesFetchedMsg:
		m.RulesLoading = false
		if msg.Err != nil {
			return m.Notify("Could not load rules: " + msg.Err.Error())
		}
		m.RulesData = msg.Data
		m.RulesIdx = 0

	case model.DeleteResultMsg:
		if msg.Err != nil {
			return m.Notify("Could not delete result: " + msg.Err.Error())
		}
		m.State = model.StateResults
		m.PrevState = model.StateResults
		render.ResizeSearchViewports(m)
		render.RefreshAndScroll(m)
		return tea.Batch(doSearch(m), m.Notify("Result deleted"))

	case model.AddResultMsg:
		if msg.Err != nil {
			m.AddStatus = "Error: " + msg.Err.Error()
		} else {
			m.AddStatus = "Added successfully!"
			for i := range m.AddInputs {
				m.AddInputs[i].SetValue("")
			}
			m.AddText.SetValue("")
		}

	case model.RulesSavedMsg:
		if msg.Err == nil {
			m.RulesLoading = true
			return tea.Batch(m.FetchRulesCmd(), m.Notify("Rules saved"))
		}
		return m.Notify("Could not save rules: " + msg.Err.Error())

	case model.ResultsMsg:
		m.IsSearching = false
		m.Results = msg.Results
		if m.SelectedIdx >= m.GetTotalResults() {
			m.SelectedIdx = m.GetTotalResults() - 1
		}
		if m.SelectedIdx < 0 && m.GetTotalResults() > 0 {
			m.SelectedIdx = 0
		}
		render.RefreshAndScroll(m)
		return network.ListenToWebSocket(m.WsChan, m.WsDone)

	case model.WsConnectedMsg:
		if msg.Conn != nil {
			m.Conn = msg.Conn
			m.WsReady = true
			m.ConnError = nil
		}
		return network.ListenToWebSocket(m.WsChan, m.WsDone)

	case model.WsDisconnectedMsg:
		m.WsReady = false
		m.IsSearching = false
		if msg.Err != nil {
			m.ConnError = msg.Err
		}
		return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg { return model.ReconnectMsg{} })

	case model.ReconnectMsg:
		return network.ConnectWebSocket(m.Cfg.WebSocketURL(), m.Cfg.BaseURL(""), m.Cfg.App.AccessToken, m.WsChan, m.WsDone)

	case model.ErrMsg:
		return tea.Batch(m.Notify(msg.Err.Error()), network.ListenToWebSocket(m.WsChan, m.WsDone))
	}
	return nil
}
