package handle

import (
	"maps"
	"testing"

	"github.com/asciimoo/hister/cmd/tui/model"
	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/indexer"

	tea "github.com/charmbracelet/bubbletea"
)

func handleTestModel(t *testing.T) *model.Model {
	t.Helper()
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("GHOSTTY_RESOURCES_DIR", "")
	cfg := config.CreateDefaultConfig()
	cfg.TUI = config.DefaultTUIConfig
	cfg.Hotkeys.TUI = maps.Clone(config.DefaultTUIHotkeys)
	m := model.InitialModel(cfg)
	Update(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

func TestSuccessfulDeleteClosesPreviewAndClearsTerminalResources(t *testing.T) {
	m := handleTestModel(t)
	m.State = model.StateDetails
	m.PrevState = model.StateResults
	m.DetailsURL = "https://example.com/article"

	if cmd := Update(m, model.DeleteResultMsg{}); cmd == nil {
		t.Fatal("successful delete returned no follow-up commands")
	}
	if m.State != model.StateResults || m.PrevState != model.StateResults {
		t.Fatalf("delete left state=%s previous=%s", m.State, m.PrevState)
	}
	if m.DetailsURL != "" {
		t.Fatalf("delete left stale preview url=%q", m.DetailsURL)
	}
}

func TestReloadDetailsPreservesOverlayState(t *testing.T) {
	m := handleTestModel(t)
	doc := &document.Document{URL: "https://example.com/new", Title: "New result"}
	m.Results = &indexer.Results{Documents: []*document.Document{doc}}
	m.SelectedIdx = 0
	m.DetailsURL = "https://example.com/old"
	m.State = model.StateContextMenu
	m.PrevState = model.StateDetails

	if cmd := ReloadDetails(m); cmd == nil {
		t.Fatal("reload returned no preview command")
	}
	if m.State != model.StateContextMenu || m.PrevState != model.StateDetails {
		t.Fatalf("reload changed overlay state=%s previous=%s", m.State, m.PrevState)
	}
	if m.DetailsURL != doc.URL || m.DetailsFocused {
		t.Fatalf("reload url=%q focused=%v", m.DetailsURL, m.DetailsFocused)
	}
}
