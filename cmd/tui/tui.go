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

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type app struct{ m *model.Model }

func (a *app) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		a.m.FetchServerConfigCmd(),
		network.ConnectWebSocket(a.m.Cfg.WebSocketURL(), a.m.Cfg.BaseURL(""), a.m.Cfg.App.AccessToken, a.m.WsChan, a.m.WsDone),
		network.ListenToWebSocket(a.m.WsChan, a.m.WsDone),
	)
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return a, handle.Update(a.m, msg)
}

func (a *app) View() string {
	return render.View(a.m)
}

func SearchTUI(cfg *config.Config) error {
	m := model.InitialModel(cfg)
	a := &app{m: m}
	p := tea.NewProgram(a, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	m.Close()
	return err
}
