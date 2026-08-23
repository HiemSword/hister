package model

import (
	"maps"
	"testing"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/indexer"
	smodel "github.com/asciimoo/hister/server/model"
)

func testConfig() *config.Config {
	cfg := config.CreateDefaultConfig()
	cfg.TUI = config.DefaultTUIConfig
	cfg.Hotkeys.TUI = maps.Clone(config.DefaultTUIHotkeys)
	cfg.SemanticSearch.Enable = true
	cfg.SemanticSearch.SemanticWeight = 0.4
	return cfg
}

func TestVisibleDocumentsMergesAndRanksSemanticResults(t *testing.T) {
	keywordOne := &document.Document{URL: "https://one.test", Domain: "one.test", Score: 10}
	keywordTwo := &document.Document{UserID: 42, URL: "https://two.test", Domain: "two.test", Score: 5}
	semanticOnly := &document.Document{URL: "https://three.test", Domain: "three.test"}
	m := InitialModel(testConfig())
	m.SemanticOn = true
	m.Results = &indexer.Results{
		History:   []*smodel.URLCount{{URL: "https://history.test"}},
		Documents: []*document.Document{keywordOne, keywordTwo},
		SemanticHits: []indexer.SemanticHit{
			{DocID: document.GetDocID(keywordTwo.UserID, keywordTwo.URL), Similarity: 0.9},
			{DocID: semanticOnly.URL, Similarity: 0.8, Document: semanticOnly},
		},
	}

	got := m.VisibleDocuments()
	want := []*document.Document{keywordTwo, keywordOne, semanticOnly}
	if len(got) != len(want) {
		t.Fatalf("VisibleDocuments length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("VisibleDocuments[%d] = %q, want %q", i, got[i].URL, want[i].URL)
		}
	}
	if m.GetTotalResults() != 4 {
		t.Fatalf("GetTotalResults() = %d, want 4", m.GetTotalResults())
	}
	m.SelectedIdx = 3
	if got := m.GetSelectedDocument(); got != semanticOnly {
		t.Fatalf("GetSelectedDocument() = %#v, want semantic-only document", got)
	}
}

func TestNavigationDefinitionsAreSelfConsistent(t *testing.T) {
	tabIDs := make(map[int]bool, len(Tabs))
	tabActions := make(map[config.Action]bool, len(Tabs))
	for _, tab := range Tabs {
		if tab.Name == "" || tabIDs[tab.ID] || tabActions[tab.Action] {
			t.Fatalf("invalid or duplicate tab definition: %#v", tab)
		}
		tabIDs[tab.ID] = true
		tabActions[tab.Action] = true
		if got, ok := TabForAction(tab.Action); !ok || got != tab.ID {
			t.Fatalf("TabForAction(%q) = %d, %v; want %d, true", tab.Action, got, ok, tab.ID)
		}
	}

	for i, option := range MenuOptions {
		got, ok := MenuOptionAt(i)
		if !ok || got != option || option.Label == "" {
			t.Fatalf("MenuOptionAt(%d) = %#v, %v; want %#v, true", i, got, ok, option)
		}
	}
	if _, ok := MenuOptionAt(-1); ok {
		t.Fatal("MenuOptionAt(-1) unexpectedly succeeded")
	}
	if _, ok := MenuOptionAt(len(MenuOptions)); ok {
		t.Fatal("MenuOptionAt(len(MenuOptions)) unexpectedly succeeded")
	}
}

func TestRulesSectionDefinitionsInitializePatternInputs(t *testing.T) {
	m := InitialModel(testConfig())
	for _, section := range RulesSections {
		if section.Aliases {
			if m.RulesPatterns(section.ID) != nil {
				t.Fatalf("alias section %d exposes pattern storage", section.ID)
			}
			continue
		}
		if section.ID < 0 || section.ID >= len(m.RulesPatternInputs) {
			t.Fatalf("pattern section id %d is outside input storage", section.ID)
		}
		if got := m.RulesPatternInputs[section.ID].Placeholder; got != section.Placeholder {
			t.Fatalf("section %d placeholder = %q, want %q", section.ID, got, section.Placeholder)
		}
		if m.RulesPatterns(section.ID) == nil {
			t.Fatalf("pattern section %d has no data storage", section.ID)
		}
	}
}

func TestRulesPatternsIncludesVersioning(t *testing.T) {
	m := InitialModel(testConfig())
	*m.RulesPatterns(RulesSectionVersioning) = append(*m.RulesPatterns(RulesSectionVersioning), "example.com/*")
	if got := m.RulesData.Versioning; len(got) != 1 || got[0] != "example.com/*" {
		t.Fatalf("versioning patterns = %#v", got)
	}
}

func TestDismissDialogReturnsToOpenDetailsPane(t *testing.T) {
	m := InitialModel(testConfig())
	m.State = StateDialog
	m.PrevState = StateDetails
	m.DetailsURL = "https://example.com/article"
	m.DialogReturnTab = -1

	m.DismissDialog()

	if m.State != StateDetails {
		t.Fatalf("dialog returned to %s, want details", m.State)
	}
}
