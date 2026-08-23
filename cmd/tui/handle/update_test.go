// SPDX-FileContributor: 4evy <git@evy.pink>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package handle

import (
	"maps"
	"testing"

	"github.com/asciimoo/hister/client"
	"github.com/asciimoo/hister/cmd/tui/model"
	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/indexer"

	tea "charm.land/bubbletea/v2"
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

func TestPasteReachesFocusedBubblesComponents(t *testing.T) {
	m := handleTestModel(t)
	if cmd := Update(m, tea.PasteMsg{Content: "pasted query"}); cmd == nil {
		t.Fatal("search paste did not schedule a search")
	}
	if got := m.TextInput.Value(); got != "pasted query" {
		t.Fatalf("search input after paste = %q", got)
	}

	m.ActiveTab = model.TabAdd
	m.State = model.StateResults
	m.AddFocusIdx = 2
	m.AddText.Focus()
	Update(m, tea.PasteMsg{Content: "first line\nsecond line"})
	if got := m.AddText.Value(); got != "first line\nsecond line" {
		t.Fatalf("textarea after paste = %q", got)
	}
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

func TestHelpOverDetailsClosesBackToResults(t *testing.T) {
	m := handleTestModel(t)
	doc := &document.Document{URL: "https://example.com/article", Title: "Article"}
	m.Results = &indexer.Results{Documents: []*document.Document{doc}}
	m.SelectedIdx = 0
	m.State = model.StateResults

	_ = OpenDetails(m)
	m.OpenOverlay(model.StateHelp)
	_ = CloseOverlay(m)
	if m.State != model.StateDetails || m.PrevState != model.StateResults {
		t.Fatalf("help returned to state=%s previous=%s", m.State, m.PrevState)
	}
	_ = CloseOverlay(m)
	if m.State != model.StateResults || m.DetailsURL != "" {
		t.Fatalf("details close left state=%s url=%q", m.State, m.DetailsURL)
	}
}

func TestPreviewRequestsAreDebouncedAndSerialized(t *testing.T) {
	m := handleTestModel(t)
	m.DetailsLoading = true
	m.DetailsURL = "https://example.com/one"
	_ = m.QueuePreviewCmd(m.DetailsURL)
	firstID := m.DetailsRequestID

	firstCmd := Update(m, model.PreviewDebounceMsg{URL: m.DetailsURL, ID: firstID})
	if firstCmd == nil || !m.DetailsFetching {
		t.Fatal("first debounced preview did not start")
	}

	m.DetailsURL = "https://example.com/two"
	_ = m.QueuePreviewCmd(m.DetailsURL)
	secondID := m.DetailsRequestID
	m.DetailsURL = "https://example.com/three"
	_ = m.QueuePreviewCmd(m.DetailsURL)
	thirdID := m.DetailsRequestID

	if cmd := Update(m, model.PreviewDebounceMsg{URL: "https://example.com/two", ID: secondID}); cmd != nil {
		t.Fatal("superseded debounce started a request")
	}
	if cmd := Update(m, model.PreviewDebounceMsg{URL: m.DetailsURL, ID: thirdID}); cmd != nil {
		t.Fatal("latest preview started while an earlier request was in flight")
	}
	if m.DetailsPendingURL != m.DetailsURL || !m.DetailsPendingReady {
		t.Fatalf("pending preview = %q ready=%v", m.DetailsPendingURL, m.DetailsPendingReady)
	}

	nextCmd := Update(m, model.PreviewFetchedMsg{URL: "https://example.com/one"})
	if nextCmd == nil || !m.DetailsFetching || m.DetailsPendingURL != "" {
		t.Fatal("latest pending preview did not start after the first request completed")
	}
	preview := &client.PreviewResponse{Title: "Latest"}
	if cmd := Update(m, model.PreviewFetchedMsg{URL: m.DetailsURL, Preview: preview}); cmd != nil {
		t.Fatal("completed latest preview unexpectedly started another request")
	}
	if m.DetailsLoading || m.DetailsFetching || m.DetailsPreview != preview {
		t.Fatalf("latest preview was not settled: loading=%v fetching=%v preview=%#v", m.DetailsLoading, m.DetailsFetching, m.DetailsPreview)
	}
}

func TestSemanticSettingsComeFromServerConfig(t *testing.T) {
	m := handleTestModel(t)
	m.Cfg.SemanticSearch.Enable = true
	m.Cfg.SemanticSearch.SemanticWeight = 0.1

	_, _ = DispatchCommonAction(m, config.ActionToggleSemantic)
	if m.SemanticOn {
		t.Fatal("local semantic configuration enabled remote search capability")
	}

	Update(m, model.ServerConfigFetchedMsg{Config: &client.ServerConfig{
		SemanticEnabled:     true,
		SemanticWeight:      0.7,
		SimilarityThreshold: 0.8,
	}})
	_, _ = DispatchCommonAction(m, config.ActionToggleSemantic)
	if !m.SemanticOn || m.SemanticWeight != 0.7 || m.SemanticThreshold != 0.8 {
		t.Fatalf("server semantic settings not applied: on=%v weight=%v threshold=%v", m.SemanticOn, m.SemanticWeight, m.SemanticThreshold)
	}
}
