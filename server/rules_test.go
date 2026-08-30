// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/testutil"
)

func TestSaveRulesPreservesOmittedRuleGroups(t *testing.T) {
	cfg, handler := newTokenTestServer(t, false)
	cfg.Rules.Skip.ReStrs = []string{"old-skip"}
	cfg.Rules.Priority.ReStrs = []string{"old-priority"}
	cfg.Rules.Versioning.ReStrs = []string{"old-versioning"}
	if err := cfg.SaveRules(); err != nil {
		t.Fatal(err)
	}

	rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/rules",
		strings.NewReader(url.Values{"skip": {"new-skip"}}.Encode()),
		map[string]string{
			"Content-Type":   "application/x-www-form-urlencoded",
			"Origin":         "hister://",
			"X-Access-Token": "secret",
		})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/rules status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	assertRulePatterns(t, "skip", cfg.Rules.Skip.ReStrs, []string{"new-skip"})
	assertRulePatterns(t, "priority", cfg.Rules.Priority.ReStrs, []string{"old-priority"})
	assertRulePatterns(t, "versioning", cfg.Rules.Versioning.ReStrs, []string{"old-versioning"})
}

func TestSaveRulesClearsExplicitEmptyRuleGroup(t *testing.T) {
	cfg, handler := newTokenTestServer(t, false)
	cfg.Rules.Skip.ReStrs = []string{"old-skip"}
	cfg.Rules.Priority.ReStrs = []string{"old-priority"}
	cfg.Rules.Versioning.ReStrs = []string{"old-versioning"}
	if err := cfg.SaveRules(); err != nil {
		t.Fatal(err)
	}

	rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/rules",
		strings.NewReader(url.Values{"versioning": {""}}.Encode()),
		map[string]string{
			"Content-Type":   "application/x-www-form-urlencoded",
			"Origin":         "hister://",
			"X-Access-Token": "secret",
		})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/rules status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	assertRulePatterns(t, "skip", cfg.Rules.Skip.ReStrs, []string{"old-skip"})
	assertRulePatterns(t, "priority", cfg.Rules.Priority.ReStrs, []string{"old-priority"})
	assertRulePatterns(t, "versioning", cfg.Rules.Versioning.ReStrs, []string{})
}

func TestSaveUserRulesPreservesOmittedRuleGroups(t *testing.T) {
	cfg := testutil.Config(t)
	cfg.App.UserHandling = true
	cfg.Server.Address = "127.0.0.1:4433"
	if err := cfg.UpdateBaseURL("http://127.0.0.1:4433"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveRules(); err != nil {
		t.Fatal(err)
	}
	cfg.Server.Database = "file::memory:"
	testutil.InitModelWithConfig(t, cfg)
	sessionStore = newSessionStore([]byte(strings.Repeat("x", 32)), cfg.BaseURL(""), sessionMaxAge)
	user := testutil.CreateUser(t, "alice")
	rules := &config.Rules{
		Skip:       &config.Rule{ReStrs: []string{"old-skip"}},
		Priority:   &config.Rule{ReStrs: []string{"old-priority"}},
		Versioning: &config.Rule{ReStrs: []string{"old-versioning"}},
		Aliases:    make(config.Aliases),
	}
	if err := rules.Compile(); err != nil {
		t.Fatal(err)
	}
	if err := model.SaveUserRules(user.ID, rules); err != nil {
		t.Fatal(err)
	}
	handler := registerEndpoints(cfg, newServerTestIndexer(t, cfg))

	rec := testutil.ServeHTTP(t, handler, http.MethodPost, "/api/rules",
		strings.NewReader(url.Values{"skip": {"new-skip"}}.Encode()),
		map[string]string{
			"Content-Type":   "application/x-www-form-urlencoded",
			"Origin":         "hister://",
			"X-Access-Token": user.Token,
		})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/rules status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	saved, err := model.GetUserRules(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertRulePatterns(t, "skip", saved.Skip.ReStrs, []string{"new-skip"})
	assertRulePatterns(t, "priority", saved.Priority.ReStrs, []string{"old-priority"})
	assertRulePatterns(t, "versioning", saved.Versioning.ReStrs, []string{"old-versioning"})
}

func assertRulePatterns(t *testing.T, label string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s rules = %#v, want %#v", label, got, want)
	}
}
