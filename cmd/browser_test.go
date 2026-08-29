package cmd

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrepareBrowserImportsContinuesAfterUnusableDatabase(t *testing.T) {
	dir := t.TempDir()
	invalidPath := filepath.Join(dir, "Chrome", "Default", "History")
	emptyPath := filepath.Join(dir, "Brave", "Default", "History")
	validPath := filepath.Join(dir, "Vivaldi", "Default", "History")

	writeBrowserHistoryFile(t, invalidPath, false, nil)
	writeBrowserHistoryFile(t, emptyPath, true, nil)
	writeBrowserHistoryFile(t, validPath, true, []string{"https://example.com"})

	choices, issues := prepareBrowserImports([]DBToImport{
		{table: "urls", databaseFile: invalidPath},
		{table: "urls", databaseFile: emptyPath},
		{table: "urls", databaseFile: validPath},
	}, 0, nil, nil)
	if len(choices) != 1 {
		t.Fatalf("prepareBrowserImports() returned %d choices, want 1", len(choices))
	}
	defer func() {
		if err := choices[0].db.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if choices[0].choice != validPath {
		t.Fatalf("prepareBrowserImports() choice = %q, want %q", choices[0].choice, validPath)
	}
	if choices[0].urls != 1 {
		t.Fatalf("prepareBrowserImports() URL count = %d, want 1", choices[0].urls)
	}

	if len(issues) != 2 {
		t.Fatalf("prepareBrowserImports() returned %d issues, want 2", len(issues))
	}
	if issues[0].databaseFile != invalidPath {
		t.Fatalf("first issue file = %q, want %q", issues[0].databaseFile, invalidPath)
	}
	if issues[1].databaseFile != emptyPath {
		t.Fatalf("second issue file = %q, want %q", issues[1].databaseFile, emptyPath)
	}
	if !errors.Is(issues[1].err, errNoBrowserURLs) {
		t.Fatalf("second issue = %v, want %v", issues[1].err, errNoBrowserURLs)
	}
}

func writeBrowserHistoryFile(t *testing.T, path string, createTable bool, urls []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if !createTable {
		return
	}
	if _, err := db.Exec("CREATE TABLE urls (url TEXT, visit_count INTEGER)"); err != nil {
		t.Fatal(err)
	}
	for _, u := range urls {
		if _, err := db.Exec("INSERT INTO urls (url, visit_count) VALUES (?, 1)", u); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBrowserImportURLQueryFlatSchemasUnchanged(t *testing.T) {
	// A regression guard, not a feature test. Introducing a FROM expression for Safari must not
	// alter a single byte of the query every other browser has always produced.
	for _, table := range []string{"urls", "moz_places", "history", "some_unknown_table"} {
		got, err := browserImportURLQuery(table, 0, nil)
		if err != nil {
			t.Fatalf("browserImportURLQuery(%q) returned error: %v", table, err)
		}
		want := "SELECT DISTINCT url FROM " + table +
			" WHERE (url LIKE 'http://%' OR url LIKE 'https://%')"
		if got != want {
			t.Fatalf("browserImportURLQuery(%q) = %q, want %q", table, got, want)
		}
	}
}

func TestBrowserImportURLQuerySafariJoinsVisits(t *testing.T) {
	got, err := browserImportURLQuery("safari", 0, nil)
	if err != nil {
		t.Fatalf("browserImportURLQuery(safari) returned error: %v", err)
	}
	for _, want := range []string{"history_items", "history_visits", "MAX(history_visits.visit_time)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("browserImportURLQuery(safari) = %q, missing %q", got, want)
		}
	}
}

func TestBrowserImportURLQuerySafariUsesCoreDataEpoch(t *testing.T) {
	// Safari counts seconds from 2001-01-01, so a Unix timestamp is 978,307,200 seconds ahead of
	// the same moment in its terms. Hard-coding the expected value rather than recomputing it
	// means a sign flip in the offset fails here rather than silently importing the wrong decade.
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got, err := browserImportURLQuery("safari", 0, &start)
	if err != nil {
		t.Fatalf("browserImportURLQuery(safari) returned error: %v", err)
	}
	const want = "AND last_visit_time >= 788918400"
	if !strings.Contains(got, want) {
		t.Fatalf("browserImportURLQuery(safari) = %q, missing %q", got, want)
	}
}

func TestPrepareBrowserImportsReadsSafariHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Safari", "History.db")
	writeSafariHistoryFile(t, path, map[string]int64{
		// visit_time values are seconds since 2001-01-01.
		"https://example.com/kept":    788918400 + 86400, // a day after the cut-off below
		"https://example.com/too-old": 788918400 - 86400,
	})

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	choices, issues := prepareBrowserImports([]DBToImport{
		{table: "safari", databaseFile: path},
	}, 0, &start, nil)
	if len(issues) != 0 {
		t.Fatalf("prepareBrowserImports() returned issues: %v", issues)
	}
	if len(choices) != 1 {
		t.Fatalf("prepareBrowserImports() returned %d choices, want 1", len(choices))
	}
	defer func() {
		if err := choices[0].db.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if choices[0].urls != 1 {
		t.Fatalf("prepareBrowserImports() URL count = %d, want 1 (the older visit should be filtered out)", choices[0].urls)
	}
}

func writeSafariHistoryFile(t *testing.T, path string, visits map[string]int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	// The two tables Safari actually uses, with only the columns the import reads.
	if _, err := db.Exec(
		"CREATE TABLE history_items (id INTEGER PRIMARY KEY, url TEXT, visit_count INTEGER)",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		"CREATE TABLE history_visits (id INTEGER PRIMARY KEY, history_item INTEGER, visit_time REAL)",
	); err != nil {
		t.Fatal(err)
	}
	id := 0
	for url, visitTime := range visits {
		id++
		if _, err := db.Exec(
			"INSERT INTO history_items (id, url, visit_count) VALUES (?, ?, 1)", id, url,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(
			"INSERT INTO history_visits (history_item, visit_time) VALUES (?, ?)", id, visitTime,
		); err != nil {
			t.Fatal(err)
		}
	}
}
