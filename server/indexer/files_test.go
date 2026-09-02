package indexer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/files"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/testutil"

	"github.com/blevesearch/bleve/v2"
)

func TestFileDocumentValidation(t *testing.T) {
	testutil.InitModel(t)
	user := testutil.CreateUser(t, "fileowner")
	dir := t.TempDir()
	idx := &Indexer{directories: []*config.Directory{{
		Path:      dir,
		Filetypes: []string{"txt"},
		Patterns:  []string{"note*"},
		User:      user.Username,
	}}}

	if err := idx.validateFileDocument(&document.Document{URL: "https://example.com"}); err != nil {
		t.Fatalf("web document rejected: %v", err)
	}
	if err := idx.validateFileDocument(&document.Document{
		URL:    files.PathToFileURL(filepath.Join(dir, "note.txt")),
		Text:   "submitted content",
		UserID: user.ID,
	}); err != nil {
		t.Fatalf("allowed file document rejected: %v", err)
	}
	if err := idx.validateFileDocument(&document.Document{
		URL:  "file:///not/configured/saved.html",
		HTML: "<html><body>submitted content</body></html>",
	}); err != nil {
		t.Fatalf("submitted HTML file rejected: %v", err)
	}

	for _, d := range []*document.Document{
		{URL: files.PathToFileURL(filepath.Join(dir, "note.txt")), UserID: user.ID},
		{URL: files.PathToFileURL(filepath.Join(dir, "note.txt")), Text: "submitted content"},
		{URL: files.PathToFileURL(filepath.Join(dir, "other.txt")), Text: "submitted content", UserID: user.ID},
		{URL: files.PathToFileURL(filepath.Join(t.TempDir(), "note.txt")), Text: "submitted content", UserID: user.ID},
	} {
		if err := idx.validateFileDocument(d); !errors.Is(err, ErrFileURLNotAllowed) {
			t.Fatalf("validation error = %v, want %v for %#v", err, ErrFileURLNotAllowed, d)
		}
	}
}

func TestAddFunctionsValidateFileDocuments(t *testing.T) {
	idx := &Indexer{}
	d := &document.Document{
		URL:       "file:///not/configured/note.txt",
		Text:      "submitted content",
		Processed: true,
	}

	if err := idx.Add(d); !errors.Is(err, ErrFileURLNotAllowed) {
		t.Fatalf("Add error = %v, want %v", err, ErrFileURLNotAllowed)
	}
	if err := newMultiBatch(idx).Add(d); !errors.Is(err, ErrFileURLNotAllowed) {
		t.Fatalf("MultiBatch.Add error = %v, want %v", err, ErrFileURLNotAllowed)
	}
}

func TestDirectoryUserResolution(t *testing.T) {
	testutil.InitModel(t)

	u1 := testutil.CreateUser(t, "alice")
	u2 := testutil.CreateUser(t, "bob")

	tests := []struct {
		name     string
		username string
		wantID   uint
		wantErr  bool
	}{
		{
			name:     "empty username is global",
			username: "",
			wantID:   0,
			wantErr:  false,
		},
		{
			name:     "existing user alice",
			username: "alice",
			wantID:   u1.ID,
			wantErr:  false,
		},
		{
			name:     "existing user bob",
			username: "bob",
			wantID:   u2.ID,
			wantErr:  false,
		},
		{
			name:     "non-existent user",
			username: "charlie",
			wantID:   0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotID uint
			var err error
			if tt.username != "" {
				u, e := model.GetUser(tt.username)
				if e != nil {
					err = e
				} else {
					gotID = u.ID
				}
			}
			if tt.wantErr {
				if err == nil {
					t.Errorf("user resolution(%q) expected error, got nil", tt.username)
				}
				return
			}
			if err != nil {
				t.Errorf("user resolution(%q) unexpected error: %v", tt.username, err)
				return
			}
			if gotID != tt.wantID {
				t.Errorf("user resolution(%q) = %d, want %d", tt.username, gotID, tt.wantID)
			}
		})
	}
}

func TestFileIndexQueueKeepsLatestPendingOperation(t *testing.T) {
	queue := newFileIndexQueue(nil)
	path := filepath.Join(t.TempDir(), "note.md")

	queue.EnqueueIndex(path, 42)
	queue.EnqueueDelete(path)

	item, ok := queue.pop()
	if !ok {
		t.Fatal("expected queued item")
	}
	if item.op != fileIndexDelete {
		t.Fatalf("queued operation = %v, want delete", item.op)
	}
	if item.path != path {
		t.Fatalf("queued path = %q, want %q", item.path, path)
	}
	if item.userID != 0 {
		t.Fatalf("queued user ID = %d, want 0", item.userID)
	}

	if _, ok := queue.pop(); ok {
		t.Fatal("expected queue to coalesce operations for the same path")
	}
}

func TestIndexFileWithUserID(t *testing.T) {
	testutil.InitModel(t)

	u := testutil.CreateUser(t, "testuser")

	testDir := t.TempDir()

	testFile := testutil.WriteFile(t, testDir, "test.txt", []byte("sample document content about indexing files for testing purposes"))
	testFile2 := testutil.WriteFile(t, testDir, "test2.txt", []byte("sample global document content for indexing test purposes"))

	idxCfg := testutil.Config(t)
	idx := newTestIndexer(t, idxCfg)
	defer idx.Close()

	if err := idx.IndexFile(testFile, u.ID); err != nil {
		t.Fatalf("IndexFile with user ID failed: %v", err)
	}

	if err := idx.IndexFile(testFile2, 0); err != nil {
		t.Fatalf("IndexFile without user ID failed: %v", err)
	}
}

func TestIndexFileAppliesDirectoryLabel(t *testing.T) {
	testDir := t.TempDir()
	testFile := testutil.WriteFile(t, testDir, "labeled.txt", []byte("sample labeled document content for indexing tests"))

	idxCfg := testutil.Config(t)
	idxCfg.Indexer.Directories = []*config.Directory{{Path: testDir, Label: "notes"}}
	idx := newTestIndexer(t, idxCfg)
	defer idx.Close()

	if err := idx.IndexFile(testFile, 0); err != nil {
		t.Fatalf("IndexFile failed: %v", err)
	}
	doc := idx.GetByURLAndUser(files.PathToFileURL(testFile), 0)
	if doc == nil {
		t.Fatal("indexed file not found")
	}
	if doc.Label != "notes" {
		t.Fatalf("Label = %q, want %q", doc.Label, "notes")
	}

	idxCfg.Indexer.Directories[0].Label = "archive"
	if err := idx.IndexFile(testFile, 0); err != nil {
		t.Fatalf("IndexFile after label change failed: %v", err)
	}
	doc = idx.GetByURLAndUser(files.PathToFileURL(testFile), 0)
	if doc.Label != "archive" {
		t.Fatalf("Label after config change = %q, want %q", doc.Label, "archive")
	}
	if doc.AddCount != 1 {
		t.Fatalf("AddCount after label change = %d, want 1", doc.AddCount)
	}
}

func TestCleanupRemovesDocumentsExcludedByConfig(t *testing.T) {
	testDir := t.TempDir()
	notePath := testutil.WriteFile(t, testDir, "note.txt", []byte("notes that should remain searchable after cleanup"))
	codePath := testutil.WriteFile(t, testDir, "main.go", []byte("package main\n\nfunc main() {}\n"))

	cfg := testutil.Config(t)
	cfg.Indexer.Directories = []*config.Directory{{Path: testDir}}
	idx := newTestIndexer(t, cfg)
	defer idx.Close()

	if err := idx.IndexFile(notePath, 0); err != nil {
		t.Fatalf("index note: %v", err)
	}
	if err := idx.IndexFile(codePath, 0); err != nil {
		t.Fatalf("index code: %v", err)
	}

	restricted := []*config.Directory{{Path: testDir, Filetypes: []string{"txt"}}}
	idx.directories = restricted
	result, err := idx.Cleanup(restricted)
	if err != nil {
		t.Fatalf("Cleanup returned an error: %v", err)
	}
	if result.LocalDocumentsChecked != 2 || result.LocalDocumentsRemoved != 1 || result.LocalDocumentsSkipped != 0 {
		t.Fatalf("cleanup result = %#v, want checked=2 removed=1 skipped=0", result)
	}
	if idx.GetByURLAndUser(files.PathToFileURL(notePath), 0) == nil {
		t.Fatal("matching note was removed")
	}
	if idx.GetByURLAndUser(files.PathToFileURL(codePath), 0) != nil {
		t.Fatal("excluded code file remains indexed")
	}
}

func TestCleanupLocalDocumentsDoesNotInspectFilesystem(t *testing.T) {
	testDir := t.TempDir()
	filePath := testutil.WriteFile(t, testDir, "note.txt", []byte("notes retained without checking their source"))
	dir := &config.Directory{Path: testDir, DeleteOnRemove: true}

	cfg := testutil.Config(t)
	cfg.Indexer.Directories = []*config.Directory{dir}
	idx := newTestIndexer(t, cfg)
	defer idx.Close()

	if err := idx.IndexFile(filePath, 0); err != nil {
		t.Fatalf("index file: %v", err)
	}
	if err := os.Remove(filePath); err != nil {
		t.Fatalf("remove source file: %v", err)
	}

	result, err := idx.CleanupLocalDocuments(cfg.Indexer.Directories)
	if err != nil {
		t.Fatalf("cleanup local documents: %v", err)
	}
	if result.Checked != 1 || result.Removed != 0 || result.Skipped != 0 {
		t.Fatalf("cleanup result = %#v, want checked=1 removed=0 skipped=0", result)
	}
	if idx.GetByURLAndUser(files.PathToFileURL(filePath), 0) == nil {
		t.Fatal("cleanup removed a document after its source file disappeared")
	}
}

func TestCleanupLocalDocumentsRemovesDocumentWithWrongOwner(t *testing.T) {
	cfg := testutil.InitModel(t)
	alice := testutil.CreateUser(t, "refresh-alice")
	bob := testutil.CreateUser(t, "refresh-bob")
	testDir := t.TempDir()
	filePath := testutil.WriteFile(t, testDir, "note.txt", []byte("notes whose configured owner changes"))
	dir := &config.Directory{Path: testDir, User: alice.Username}
	cfg.Indexer.Directories = []*config.Directory{dir}

	idx := newTestIndexer(t, cfg)
	defer idx.Close()
	if err := idx.IndexFile(filePath, alice.ID); err != nil {
		t.Fatalf("index file for alice: %v", err)
	}

	dir.User = bob.Username
	result, err := idx.CleanupLocalDocuments(cfg.Indexer.Directories)
	if err != nil {
		t.Fatalf("cleanup after owner change: %v", err)
	}
	if result.Checked != 1 || result.Removed != 1 || result.Skipped != 0 {
		t.Fatalf("cleanup result = %#v, want checked=1 removed=1 skipped=0", result)
	}
	fileURL := files.PathToFileURL(filePath)
	if idx.GetByURLAndUser(fileURL, alice.ID) != nil {
		t.Fatal("document remains indexed for its previous owner")
	}
	if idx.GetByURLAndUser(fileURL, bob.ID) != nil {
		t.Fatal("cleanup unexpectedly indexed a replacement document")
	}
}

func TestCleanupLocalDocumentsPreservesDocumentWhenOwnerCannotBeResolved(t *testing.T) {
	cfg := testutil.InitModel(t)
	alice := testutil.CreateUser(t, "cleanup-owner-alice")
	testDir := t.TempDir()
	filePath := testutil.WriteFile(t, testDir, "note.txt", []byte("notes preserved when ownership cannot be checked"))
	dir := &config.Directory{Path: testDir, User: alice.Username}
	cfg.Indexer.Directories = []*config.Directory{dir}

	idx := newTestIndexer(t, cfg)
	defer idx.Close()
	if err := idx.IndexFile(filePath, alice.ID); err != nil {
		t.Fatalf("index file for alice: %v", err)
	}

	dir.User = "missing-cleanup-owner"
	result, err := idx.CleanupLocalDocuments(cfg.Indexer.Directories)
	if err != nil {
		t.Fatalf("cleanup with unresolved owner: %v", err)
	}
	if result.Checked != 1 || result.Removed != 0 || result.Skipped != 1 {
		t.Fatalf("cleanup result = %#v, want checked=1 removed=0 skipped=1", result)
	}
	if idx.GetByURLAndUser(files.PathToFileURL(filePath), alice.ID) == nil {
		t.Fatal("cleanup removed a document whose configured owner could not be resolved")
	}
}

func TestCleanupLocalDocumentsDeletesMultipleBatches(t *testing.T) {
	testDir := t.TempDir()
	cfg := testutil.Config(t)
	cfg.Indexer.Directories = []*config.Directory{{Path: testDir}}
	idx := newTestIndexer(t, cfg)
	defer idx.Close()

	batch := idx.NewMultiBatch()
	for n := range 201 {
		doc := &document.Document{
			URL:       files.PathToFileURL(filepath.Join(testDir, fmt.Sprintf("file-%03d.go", n))),
			Text:      "indexed code file",
			Type:      document.Local,
			Processed: true,
		}
		if err := batch.Add(doc); err != nil {
			t.Fatalf("add document %d to batch: %v", n, err)
		}
	}
	if err := batch.Save(); err != nil {
		t.Fatalf("save document batch: %v", err)
	}

	restricted := []*config.Directory{{Path: testDir, Filetypes: []string{"txt"}}}
	result, err := idx.CleanupLocalDocuments(restricted)
	if err != nil {
		t.Fatalf("cleanup multiple batches: %v", err)
	}
	if result.Checked != 201 || result.Removed != 201 || result.Skipped != 0 {
		t.Fatalf("cleanup result = %#v, want checked=201 removed=201 skipped=0", result)
	}
	if idx.GetByURLAndUser(files.PathToFileURL(filepath.Join(testDir, "file-200.go")), 0) != nil {
		t.Fatal("document from the second cleanup batch remains indexed")
	}
}

func TestReindexSkipsLocalFilesExcludedByConfig(t *testing.T) {
	testDir := t.TempDir()
	notePath := testutil.WriteFile(t, testDir, "note.txt", []byte("notes that should survive a restricted reindex"))
	codePath := testutil.WriteFile(t, testDir, "main.go", []byte("package main\n\nfunc main() {}\n"))

	cfg := testutil.Config(t)
	cfg.Indexer.Directories = []*config.Directory{{Path: testDir}}
	idx := newTestIndexer(t, cfg)
	defer idx.Close()

	if err := idx.IndexFile(notePath, 0); err != nil {
		t.Fatalf("index note: %v", err)
	}
	if err := idx.IndexFile(codePath, 0); err != nil {
		t.Fatalf("index code: %v", err)
	}

	restricted := []*config.Directory{{Path: testDir, Filetypes: []string{"txt"}}}
	if err := idx.Reindex(&config.Rules{}, false, cfg.Indexer.DetectLanguages, cfg.Indexer.KeepStopwords, restricted); err != nil {
		t.Fatalf("Reindex returned an error: %v", err)
	}
	if idx.GetByURLAndUser(files.PathToFileURL(notePath), 0) == nil {
		t.Fatal("matching note was removed during reindex")
	}
	if idx.GetByURLAndUser(files.PathToFileURL(codePath), 0) != nil {
		t.Fatal("excluded code file remains indexed after reindex")
	}
}

func TestReindexReloadsMarkdownFile(t *testing.T) {
	testDir := t.TempDir()
	const initialTitle = "old old old old old"
	const title = "test test test test test"
	markdownPath := testutil.WriteFile(t, testDir, "heading.md", []byte("# "+initialTitle+"\n"))

	cfg := testutil.Config(t)
	cfg.Indexer.Directories = []*config.Directory{{Path: testDir}}
	idx := newTestIndexer(t, cfg)
	defer idx.Close()

	if err := idx.IndexFile(markdownPath, 0); err != nil {
		t.Fatalf("index Markdown file: %v", err)
	}
	url := files.PathToFileURL(markdownPath)
	before := idx.GetByURLAndUser(url, 0)
	if before == nil {
		t.Fatal("indexed Markdown file not found")
	}
	testutil.WriteFile(t, testDir, "heading.md", []byte("# "+title+"\n"))
	updated := time.Unix(before.Updated+10, 0)
	if err := os.Chtimes(markdownPath, updated, updated); err != nil {
		t.Fatalf("set Markdown modification time: %v", err)
	}

	if err := idx.Reindex(&config.Rules{}, false, cfg.Indexer.DetectLanguages, cfg.Indexer.KeepStopwords, cfg.Indexer.Directories); err != nil {
		t.Fatalf("reindex Markdown file: %v", err)
	}

	got := idx.GetByURLAndUser(url, 0)
	if got == nil {
		t.Fatal("Markdown file not found after reindex")
	}
	if got.Type != document.Local {
		t.Fatalf("document type = %v, want %v", got.Type, document.Local)
	}
	if got.Title != title {
		t.Fatalf("title = %q, want %q", got.Title, title)
	}
	if got.Text != title {
		t.Fatalf("text = %q, want %q", got.Text, title)
	}
	if !strings.Contains(got.HTML, "<h1") {
		t.Fatalf("HTML = %q, want rendered Markdown heading", got.HTML)
	}
	if got.Metadata["type"] != "markdown" {
		t.Fatalf("metadata type = %v, want markdown", got.Metadata["type"])
	}
	if got.Added != before.Added || got.Updated != updated.Unix() || got.AddCount != before.AddCount {
		t.Fatalf(
			"document metadata after reindex = added %d, updated %d, count %d; want %d, %d, %d",
			got.Added,
			got.Updated,
			got.AddCount,
			before.Added,
			updated.Unix(),
			before.AddCount,
		)
	}
}

func TestAddDocumentIncrementsAddCount(t *testing.T) {
	idxCfg := testutil.Config(t)
	idx := newTestIndexer(t, idxCfg)
	defer idx.Close()

	url := "https://example.com/count"
	for range 2 {
		if err := idx.Add(&document.Document{
			URL:   url,
			Title: "Counted",
			Text:  "Counted document text",
		}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	got := idx.GetByURLAndUser(url, 0)
	if got == nil {
		t.Fatal("document not found")
	}
	if got.AddCount != 2 {
		t.Fatalf("AddCount = %d, want 2", got.AddCount)
	}

	latest := idx.GetLatestDocuments(10, "", 0)
	if latest == nil {
		t.Fatal("latest documents not found")
	}
	if len(latest.Documents) != 1 {
		t.Fatalf("latest documents count = %d, want 1", len(latest.Documents))
	}
	if latest.Documents[0].AddCount != 2 {
		t.Fatalf("latest AddCount = %d, want 2", latest.Documents[0].AddCount)
	}
}

func TestAddDocumentTimestamps(t *testing.T) {
	idxCfg := testutil.Config(t)
	idx := newTestIndexer(t, idxCfg)
	defer idx.Close()

	url := "https://example.com/timestamps"
	if err := idx.Add(&document.Document{
		URL:   url,
		Title: "Initial document",
		Text:  "Initial document text",
	}); err != nil {
		t.Fatalf("initial Add failed: %v", err)
	}

	initial := idx.GetByURLAndUser(url, 0)
	if initial == nil {
		t.Fatal("initial document not found")
	}
	if initial.Added == 0 {
		t.Fatal("Added was not populated")
	}
	if initial.Updated == 0 {
		t.Fatal("Updated was not populated")
	}

	if err := idx.Add(&document.Document{
		URL:       url,
		Title:     "Updated document",
		Text:      "Updated document text",
		Added:     initial.Added + 10,
		Updated:   initial.Updated + 20,
		Processed: true,
	}); err != nil {
		t.Fatalf("second Add failed: %v", err)
	}

	updated := idx.GetByURLAndUser(url, 0)
	if updated == nil {
		t.Fatal("updated document not found")
	}
	if updated.Added != initial.Added {
		t.Fatalf("Added = %d, want %d", updated.Added, initial.Added)
	}
	if updated.Updated != initial.Updated+20 {
		t.Fatalf("Updated = %d, want %d", updated.Updated, initial.Updated+20)
	}
}

func TestAddDocumentAndMultiBatchUseSamePreparation(t *testing.T) {
	for _, batched := range []bool{false, true} {
		name := "direct"
		if batched {
			name = "batch"
		}
		t.Run(name, func(t *testing.T) {
			idx := newTestIndexer(t, testutil.Config(t))
			defer idx.Close()

			url := "https://example.com/shared-preparation"
			initial := &document.Document{
				URL:      url,
				Title:    "Initial document",
				Text:     "Initial document text",
				HTML:     "<p>old preview</p>",
				Language: "en",
				Label:    "preserved label",
				Added:    100,
				AddCount: 2,
			}
			if err := idx.save(initial); err != nil {
				t.Fatalf("save initial document: %v", err)
			}
			oldHTMLKey := initial.HTMLKey

			updated := &document.Document{
				URL:       url,
				Title:     "Updated document",
				Text:      "Updated document text",
				HTML:      "<p>new preview</p>",
				Updated:   200,
				Processed: true,
			}
			if batched {
				batch := idx.NewMultiBatch()
				if err := batch.Add(updated); err != nil {
					t.Fatalf("add document to batch: %v", err)
				}
				if err := batch.Save(); err != nil {
					t.Fatalf("save batch: %v", err)
				}
			} else if err := idx.Add(updated); err != nil {
				t.Fatalf("add document: %v", err)
			}

			got := idx.GetByURLAndUser(url, 0)
			if got == nil {
				t.Fatal("updated document not found")
			}
			if got.Added != 100 || got.Updated != 200 {
				t.Fatalf("timestamps are Added=%d Updated=%d, want 100 and 200", got.Added, got.Updated)
			}
			if got.AddCount != 3 {
				t.Fatalf("AddCount = %d, want 3", got.AddCount)
			}
			if got.Label != "preserved label" {
				t.Fatalf("Label = %q, want preserved label", got.Label)
			}
			if got.Language != "" {
				t.Fatalf("Language = %q, want the default index", got.Language)
			}
			if got.HTML != "<p>new preview</p>" {
				t.Fatalf("HTML = %q, want updated preview", got.HTML)
			}
			if copies := countDocIDCopies(t, idx, document.GetDocID(0, url)); copies != 1 {
				t.Fatalf("document copies = %d, want 1", copies)
			}
			if _, err := os.Stat(dataFilePath(idx.dir, htmlSubdir, oldHTMLKey)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("old preview file stat error = %v, want not found", err)
			}
		})
	}
}

func TestInitBackfillsLegacyUpdatedTimestamp(t *testing.T) {
	idxCfg := testutil.Config(t)
	svc := newTestIndexer(t, idxCfg)

	url := "https://example.com/legacy-updated"
	id := document.GetDocID(0, url)
	legacy := map[string]any{
		"url":            url,
		"title":          "Legacy document",
		"text":           "Legacy document text",
		"domain":         "example.com",
		"added":          int64(1234),
		"type":           int64(0),
		"user_id":        int64(0),
		"language":       "",
		"add_count":      int64(1),
		"metadata.topic": "compatibility",
	}
	subIndex := svc.indexers[defaultIndexerName]
	if err := subIndex.Index(id, legacy); err != nil {
		t.Fatalf("failed to index legacy document: %v", err)
	}
	if err := subIndex.DeleteInternal([]byte(updatedBackfillKey)); err != nil {
		t.Fatalf("failed to clear backfill marker: %v", err)
	}
	svc.Close()

	svc = newTestIndexer(t, idxCfg)
	defer svc.Close()
	if err := svc.waitForMaintenance(); err != nil {
		t.Fatalf("updated timestamp backfill failed: %v", err)
	}

	doc := svc.GetByURLAndUser(url, 0)
	if doc == nil {
		t.Fatal("backfilled document not found")
	}
	if doc.Updated != doc.Added || doc.Updated != 1234 {
		t.Fatalf("timestamps are Added=%d Updated=%d, want both 1234", doc.Added, doc.Updated)
	}
	if doc.Metadata["topic"] != "compatibility" {
		t.Fatalf("metadata topic = %#v, want compatibility", doc.Metadata["topic"])
	}
	marker, err := svc.indexers[defaultIndexerName].GetInternal([]byte(updatedBackfillKey))
	if err != nil {
		t.Fatalf("failed to read backfill marker: %v", err)
	}
	if len(marker) == 0 {
		t.Fatal("backfill marker was not stored")
	}
}

func TestUpdatedTimestampBackfillProcessesMultipleBatches(t *testing.T) {
	idx := newTestIndexer(t, testutil.Config(t))
	defer idx.Close()
	if err := idx.waitForMaintenance(); err != nil {
		t.Fatalf("initial updated timestamp backfill failed: %v", err)
	}

	subIndex := idx.indexers[defaultIndexerName]
	batch := subIndex.NewBatch()
	urls := make([]string, 5)
	for n := range urls {
		urls[n] = fmt.Sprintf("https://example.com/legacy-updated/%d", n)
		legacy := map[string]any{
			"url":       urls[n],
			"title":     fmt.Sprintf("Legacy document %d", n),
			"added":     int64(1000 + n),
			"type":      int64(0),
			"user_id":   int64(0),
			"add_count": int64(1),
		}
		if err := batch.Index(document.GetDocID(0, urls[n]), legacy); err != nil {
			t.Fatalf("failed to prepare legacy document: %v", err)
		}
	}
	if err := subIndex.Batch(batch); err != nil {
		t.Fatalf("failed to index legacy documents: %v", err)
	}
	if err := subIndex.DeleteInternal([]byte(updatedBackfillKey)); err != nil {
		t.Fatalf("failed to clear backfill marker: %v", err)
	}

	if err := idx.backfillUpdatedTimestamps(context.Background(), 2); err != nil {
		t.Fatalf("updated timestamp backfill failed: %v", err)
	}
	for n, url := range urls {
		doc := idx.GetByURLAndUser(url, 0)
		if doc == nil {
			t.Fatalf("backfilled document %d not found", n)
		}
		want := int64(1000 + n)
		if doc.Added != want || doc.Updated != want {
			t.Fatalf("document %d timestamps are Added=%d Updated=%d, want both %d", n, doc.Added, doc.Updated, want)
		}
	}
	marker, err := subIndex.GetInternal([]byte(updatedBackfillKey))
	if err != nil {
		t.Fatalf("failed to read backfill marker: %v", err)
	}
	if len(marker) == 0 {
		t.Fatal("backfill marker was not stored")
	}
}

func TestUpdatedTimestampBackfillHonorsCancellation(t *testing.T) {
	idx := newTestIndexer(t, testutil.Config(t))
	defer idx.Close()
	if err := idx.waitForMaintenance(); err != nil {
		t.Fatalf("initial updated timestamp backfill failed: %v", err)
	}

	subIndex := idx.indexers[defaultIndexerName]
	if err := subIndex.DeleteInternal([]byte(updatedBackfillKey)); err != nil {
		t.Fatalf("failed to clear backfill marker: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := idx.backfillUpdatedTimestamps(ctx, 2); !errors.Is(err, context.Canceled) {
		t.Fatalf("backfill error = %v, want context.Canceled", err)
	}
	marker, err := subIndex.GetInternal([]byte(updatedBackfillKey))
	if err != nil {
		t.Fatalf("failed to read backfill marker: %v", err)
	}
	if len(marker) != 0 {
		t.Fatal("canceled backfill stored its completion marker")
	}
}

func TestUpdatedTimestampBackfillHandlesMissingAddedTimestamp(t *testing.T) {
	idx := newTestIndexer(t, testutil.Config(t))
	defer idx.Close()
	if err := idx.waitForMaintenance(); err != nil {
		t.Fatalf("initial updated timestamp backfill failed: %v", err)
	}

	subIndex := idx.indexers[defaultIndexerName]
	url := "https://example.com/legacy-without-added"
	legacy := map[string]any{
		"url":       url,
		"title":     "Legacy document without added timestamp",
		"type":      int64(0),
		"user_id":   int64(0),
		"add_count": int64(1),
	}
	if err := subIndex.Index(document.GetDocID(0, url), legacy); err != nil {
		t.Fatalf("failed to index legacy document: %v", err)
	}
	if err := subIndex.DeleteInternal([]byte(updatedBackfillKey)); err != nil {
		t.Fatalf("failed to clear backfill marker: %v", err)
	}

	if err := idx.backfillUpdatedTimestamps(context.Background(), 2); err != nil {
		t.Fatalf("updated timestamp backfill failed: %v", err)
	}
	updatedExists := bleve.NewNumericRangeQuery(nil, nil)
	updatedExists.SetField("updated")
	result, err := subIndex.Search(bleve.NewSearchRequest(updatedExists))
	if err != nil {
		t.Fatalf("failed to find updated timestamp: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("documents with an updated timestamp = %d, want 1", result.Total)
	}
	marker, err := subIndex.GetInternal([]byte(updatedBackfillKey))
	if err != nil {
		t.Fatalf("failed to read backfill marker: %v", err)
	}
	if len(marker) == 0 {
		t.Fatal("backfill marker was not stored")
	}
}

func TestUpdatedControlsDateSearch(t *testing.T) {
	idxCfg := testutil.Config(t)
	idx := newTestIndexer(t, idxCfg)
	defer idx.Close()

	docs := []*document.Document{
		{
			URL:       "https://example.com/recently-added",
			Title:     "Recently added",
			Text:      "Recently added document text",
			Added:     200,
			Updated:   250,
			Processed: true,
		},
		{
			URL:       "https://example.com/recently-updated",
			Title:     "Recently updated",
			Text:      "Recently updated document text",
			Added:     100,
			Updated:   300,
			Processed: true,
		},
	}
	for _, doc := range docs {
		if err := idx.Add(doc); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	res, err := idx.Search(&Query{MatchAll: true, Sort: "date", DateFrom: 275})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(res.Documents) != 1 {
		t.Fatalf("document count = %d, want 1", len(res.Documents))
	}
	if res.Documents[0].URL != docs[1].URL {
		t.Fatalf("first document URL = %q, want %q", res.Documents[0].URL, docs[1].URL)
	}

	latest := idx.GetLatestDocuments(10, "", 0)
	if latest == nil || len(latest.Documents) != 2 {
		t.Fatalf("latest documents = %#v, want two documents", latest)
	}
	if latest.Documents[0].URL != docs[1].URL {
		t.Fatalf("latest document URL = %q, want %q", latest.Documents[0].URL, docs[1].URL)
	}
}

func TestIndexFileUsesModificationTimeAsUpdated(t *testing.T) {
	testDir := t.TempDir()
	testFile := testutil.WriteFile(t, testDir, "timestamp.txt", []byte("document content with a source modification time"))
	modTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(testFile, modTime, modTime); err != nil {
		t.Fatalf("failed to set file modification time: %v", err)
	}

	idxCfg := testutil.Config(t)
	idx := newTestIndexer(t, idxCfg)
	defer idx.Close()

	if err := idx.IndexFile(testFile, 0); err != nil {
		t.Fatalf("IndexFile failed: %v", err)
	}
	doc := idx.GetByURLAndUser(files.PathToFileURL(testFile), 0)
	if doc == nil {
		t.Fatal("indexed file not found")
	}
	if doc.Updated != modTime.Unix() {
		t.Fatalf("Updated = %d, want %d", doc.Updated, modTime.Unix())
	}
	if doc.Added == 0 || doc.Added == doc.Updated {
		t.Fatalf("Added = %d, want an indexing timestamp distinct from the file modification time", doc.Added)
	}
	if err := idx.IndexFile(testFile, 0); err != nil {
		t.Fatalf("second IndexFile failed: %v", err)
	}
	doc = idx.GetByURLAndUser(files.PathToFileURL(testFile), 0)
	if doc.AddCount != 1 {
		t.Fatalf("AddCount = %d, want unchanged file to be skipped", doc.AddCount)
	}
}

func TestAddDocumentDoesNotIncrementAddCountForExtraDocuments(t *testing.T) {
	idxCfg := testutil.Config(t)
	idx := newTestIndexer(t, idxCfg)
	defer idx.Close()

	parentURL := "https://example.com/parent"
	extraURL := "https://example.com/extra"
	for range 2 {
		if err := idx.Add(&document.Document{
			URL:   parentURL,
			Title: "Parent",
			Text:  "Parent document text",
			ExtraDocuments: []*document.Document{
				{
					URL:   extraURL,
					Title: "Extra",
					Text:  "Extra document text",
				},
			},
		}); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	parent := idx.GetByURLAndUser(parentURL, 0)
	if parent == nil {
		t.Fatal("parent document not found")
	}
	if parent.AddCount != 2 {
		t.Fatalf("parent AddCount = %d, want 2", parent.AddCount)
	}

	extra := idx.GetByURLAndUser(extraURL, 0)
	if extra == nil {
		t.Fatal("extra document not found")
	}
	if extra.AddCount != 1 {
		t.Fatalf("extra AddCount = %d, want 1", extra.AddCount)
	}
}

func TestMultiBatchAddsExtraDocuments(t *testing.T) {
	idxCfg := testutil.Config(t)
	idx := newTestIndexer(t, idxCfg)
	defer idx.Close()

	parentURL := "https://example.com/batch-parent"
	extraURL := "https://example.com/batch-extra"
	batch := idx.NewMultiBatch()
	if err := batch.Add(&document.Document{
		URL:   parentURL,
		Title: "Parent",
		Text:  "Parent document text",
		ExtraDocuments: []*document.Document{
			{
				URL:   extraURL,
				Title: "Extra",
				Text:  "Extra document text",
			},
		},
	}); err != nil {
		t.Fatalf("batch add failed: %v", err)
	}
	if err := batch.Save(); err != nil {
		t.Fatalf("batch save failed: %v", err)
	}

	if idx.GetByURLAndUser(parentURL, 0) == nil {
		t.Fatal("parent document not found")
	}
	extra := idx.GetByURLAndUser(extraURL, 0)
	if extra == nil {
		t.Fatal("extra document not found")
	}
	if extra.AddCount != 1 {
		t.Fatalf("extra AddCount = %d, want 1", extra.AddCount)
	}
}

func TestAddDocumentTreatsMissingAddCountAsOne(t *testing.T) {
	idxCfg := testutil.Config(t)
	idx := newTestIndexer(t, idxCfg)
	defer idx.Close()

	url := "https://example.com/legacy-count"
	err := idx.save(&document.Document{
		URL:   url,
		Title: "Legacy counted",
		Text:  "Legacy counted document text",
	})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	got := idx.GetByURLAndUser(url, 0)
	if got == nil {
		t.Fatal("document not found")
	}
	if got.AddCount != 1 {
		t.Fatalf("legacy AddCount = %d, want 1", got.AddCount)
	}

	err = idx.Add(&document.Document{
		URL:   url,
		Title: "Legacy counted",
		Text:  "Legacy counted document text",
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	got = idx.GetByURLAndUser(url, 0)
	if got == nil {
		t.Fatal("document not found after add")
	}
	if got.AddCount != 2 {
		t.Fatalf("AddCount after add = %d, want 2", got.AddCount)
	}
}

func TestAddDocumentReusesExistingDocumentLookup(t *testing.T) {
	idxCfg := testutil.Config(t)
	idx := newTestIndexer(t, idxCfg)
	defer idx.Close()

	url := "https://example.com/reused-lookup"
	if err := idx.save(&document.Document{
		URL:      url,
		Title:    "Existing document",
		Text:     "Existing document text",
		Label:    "preserved label",
		AddCount: 2,
	}); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}

	searchesBefore := indexSearchCount(t, idx.indexers[defaultIndexerName])
	if err := idx.Add(&document.Document{
		URL:   url,
		Title: "Updated document",
		Text:  "Updated document text",
	}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	searches := indexSearchCount(t, idx.indexers[defaultIndexerName]) - searchesBefore
	if searches != 1 {
		t.Fatalf("existing document searches = %d, want 1", searches)
	}

	got := idx.GetByURLAndUser(url, 0)
	if got == nil {
		t.Fatal("document not found after add")
	}
	if got.AddCount != 3 {
		t.Fatalf("AddCount = %d, want 3", got.AddCount)
	}
	if got.Label != "preserved label" {
		t.Fatalf("Label = %q, want %q", got.Label, "preserved label")
	}
}

func TestSaveRemovesStaleLanguageCopy(t *testing.T) {
	idxCfg := testutil.Config(t)
	idx := newTestIndexer(t, idxCfg)
	defer idx.Close()

	url := "https://example.com/language-copy"
	err := idx.save(&document.Document{
		URL:      url,
		Title:    "Language copy",
		Text:     "Language copy text",
		Language: "en",
		AddCount: 4,
	})
	if err != nil {
		t.Fatalf("first save failed: %v", err)
	}
	if copies := countDocIDCopies(t, idx, document.GetDocID(0, url)); copies != 1 {
		t.Fatalf("copies after first save = %d, want 1", copies)
	}

	staleIndex := idx.indexers[indexNameForLanguage("en")]
	unrelated := idx.getOrCreate("fr")
	staleDeletesBefore := indexDeleteCount(t, staleIndex)
	unrelatedDeletesBefore := indexDeleteCount(t, unrelated)

	err = idx.save(&document.Document{
		URL:      url,
		Title:    "Language copy",
		Text:     "Language copy text",
		Language: "",
		AddCount: 5,
	})
	if err != nil {
		t.Fatalf("second save failed: %v", err)
	}
	if copies := countDocIDCopies(t, idx, document.GetDocID(0, url)); copies != 1 {
		t.Fatalf("copies after language change = %d, want 1", copies)
	}

	got := idx.GetByURLAndUser(url, 0)
	if got == nil {
		t.Fatal("document not found")
	}
	if got.AddCount != 5 {
		t.Fatalf("AddCount = %d, want 5", got.AddCount)
	}
	staleDeletes := indexDeleteCount(t, staleIndex) - staleDeletesBefore
	if staleDeletes != 1 {
		t.Fatalf("stale index delete count = %d, want 1", staleDeletes)
	}
	unrelatedDeletes := indexDeleteCount(t, unrelated) - unrelatedDeletesBefore
	if unrelatedDeletes != 0 {
		t.Fatalf("unrelated index delete count = %d, want 0", unrelatedDeletes)
	}
}

func indexDeleteCount(t *testing.T, idx bleve.Index) uint64 {
	t.Helper()
	stats := idx.StatsMap()
	indexStats, ok := stats["index"].(map[string]any)
	if !ok {
		t.Fatalf("index stats have type %T, want map[string]any", stats["index"])
	}
	deletes, ok := indexStats["deletes"].(uint64)
	if !ok {
		t.Fatalf("delete count has type %T, want uint64", indexStats["deletes"])
	}
	return deletes
}

func indexSearchCount(t *testing.T, idx bleve.Index) uint64 {
	t.Helper()
	stats := idx.StatsMap()
	searches, ok := stats["searches"].(uint64)
	if !ok {
		t.Fatalf("search count has type %T, want uint64", stats["searches"])
	}
	return searches
}

func countDocIDCopies(t *testing.T, idx *Indexer, id string) uint64 {
	t.Helper()
	q := bleve.NewDocIDQuery([]string{id})
	req := bleve.NewSearchRequest(q)
	req.Size = 10
	res, err := idx.idx.Search(req)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	return res.Total
}

func TestDirectoryUserField(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		expected string
	}{
		{
			name:     "empty user",
			user:     "",
			expected: "",
		},
		{
			name:     "user set",
			user:     "alice",
			expected: "alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := &config.Directory{
				Path: "/some/path",
				User: tt.user,
			}
			if dir.User != tt.expected {
				t.Errorf("Directory.User = %q, want %q", dir.User, tt.expected)
			}
		})
	}
}
