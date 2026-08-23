// SPDX-FileContributor: FlameFlag <github@flameflag.dev>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"github.com/asciimoo/hister/cmd/tui/handle"
	"github.com/asciimoo/hister/cmd/tui/model"
	"github.com/asciimoo/hister/cmd/tui/network"
	"github.com/asciimoo/hister/cmd/tui/render"
	"github.com/asciimoo/hister/config"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type app struct{ m *model.Model }

func (a *app) Init() tea.Cmd {
	cmds := []tea.Cmd{
		textinput.Blink,
		a.m.FetchServerConfigCmd(),
		tea.RequestBackgroundColor,
		network.ConnectWebSocket(a.m.Cfg.WebSocketURL(), a.m.Cfg.BaseURL(""), a.m.Cfg.App.AccessToken, a.m.WsChan, a.m.WsDone),
		network.ListenToWebSocket(a.m.WsChan, a.m.WsDone),
	}
	return tea.Batch(cmds...)
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return a, handle.Update(a.m, msg)
}

func (a *app) View() tea.View {
	v := tea.NewView(render.View(a.m))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "Hister"
	if a.m.ThemeName != "no-color" {
		v.BackgroundColor = a.m.BackgroundColor
		v.ForegroundColor = a.m.ForegroundColor
	}
	return v
}

func SearchTUI(cfg *config.Config) error {
	m := model.InitialModel(cfg)
	a := &app{m: m}
	p := tea.NewProgram(a)
	_, err := p.Run()
	m.Close()
	return err
}
