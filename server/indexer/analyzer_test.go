package indexer

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/blevesearch/bleve/v2"
)

func analyzedTerms(t *testing.T, language string, keepStopwords bool, input string) []string {
	t.Helper()
	mapping := createMapping(language, keepStopwords)
	analyzerName := mapping.AnalyzerNameForPath("text")
	analyzer := mapping.AnalyzerNamed(analyzerName)
	if analyzer == nil {
		t.Fatalf("analyzer %q is unavailable", analyzerName)
	}
	tokens := analyzer.Analyze([]byte(input))
	terms := make([]string, len(tokens))
	for i, token := range tokens {
		terms[i] = string(token.Term)
	}
	return terms
}

func TestEnglishAnalyzerKeepStopwords(t *testing.T) {
	withoutStopwords := analyzedTerms(t, "en", false, "for your information")
	if slices.Contains(withoutStopwords, "for") || slices.Contains(withoutStopwords, "your") {
		t.Fatalf("standard analyzer retained stopwords: %v", withoutStopwords)
	}

	withStopwords := analyzedTerms(t, "en", true, "for your information")
	want := []string{"for", "your", "inform"}
	if !slices.Equal(withStopwords, want) {
		t.Fatalf("keep stopwords analyzer terms = %v, want %v", withStopwords, want)
	}
}

func TestProcessedFieldIsNotStoredOrIndexed(t *testing.T) {
	fieldMapping := createMapping("default", false).FieldMappingForPath("processed")
	if fieldMapping.Store {
		t.Error("processed field is stored")
	}
	if fieldMapping.Index {
		t.Error("processed field is indexed")
	}
	if fieldMapping.DocValues {
		t.Error("processed field has doc values")
	}
}

func TestLanguageIndexesAreReindexSourcesWhenDetectionIsDisabled(t *testing.T) {
	dir := t.TempDir()
	idx, err := initializeIndexer(dir, true, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.addIndexer(indexNameForLanguage("en"), "en"); err != nil {
		idx.Close()
		t.Fatal(err)
	}
	languageIndex := idx.indexers[indexNameForLanguage("en")]
	if err := languageIndex.Index("english-document", map[string]string{
		"title": "English document",
		"text":  "Document stored in a language index",
	}); err != nil {
		idx.Close()
		t.Fatal(err)
	}
	idx.Close()

	idx, err = initializeIndexer(dir, false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	if _, exists := idx.indexers[indexNameForLanguage("en")]; exists {
		t.Fatal("English index is active with language detection disabled")
	}
	activeCount, err := idx.idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if activeCount != 0 {
		t.Fatalf("active index document count = %d, want 0", activeCount)
	}

	sources, closeSources, err := openReindexSources(dir, idx.indexers)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSources()
	source, exists := sources[indexNameForLanguage("en")]
	if !exists {
		t.Fatal("English index is unavailable as a reindex source")
	}
	sourceCount, err := source.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if sourceCount != 1 {
		t.Fatalf("English source document count = %d, want 1", sourceCount)
	}
}

func TestLanguageIndexNameValidation(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"index_en.db":        true,
		"index_zh.db":        true,
		"index_cjk.db":       true,
		"index.db":           false,
		"index_.db":          false,
		"index_EN.db":        false,
		"index_zz.db":        false,
		"index_backup.db":    false,
		"index_en.backup.db": false,
		"prefix_index_en.db": false,
	}
	for name, want := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := isLanguageIndexName(name); got != want {
				t.Fatalf("isLanguageIndexName(%q) = %v, want %v", name, got, want)
			}
		})
	}
}

func TestInitializeIndexerIgnoresLanguageIndexLookalike(t *testing.T) {
	dir := t.TempDir()
	writeInvalidIndexLookalike(t, dir)

	idx, err := initializeIndexer(dir, true, false, "")
	if err != nil {
		t.Fatalf("initialize indexer: %v", err)
	}
	defer idx.Close()
	if _, exists := idx.indexers["index_backup.db"]; exists {
		t.Fatal("language index lookalike was opened")
	}
}

func TestOpenReindexSourcesIgnoresLanguageIndexLookalike(t *testing.T) {
	dir := t.TempDir()
	writeInvalidIndexLookalike(t, dir)

	sources, closeSources, err := openReindexSources(dir, map[string]bleve.Index{})
	if err != nil {
		t.Fatalf("open reindex sources: %v", err)
	}
	defer closeSources()
	if len(sources) != 0 {
		t.Fatalf("reindex sources = %v, want none", sources)
	}
}

func writeInvalidIndexLookalike(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "index_backup.db")
	if err := os.WriteFile(path, []byte("not a Bleve index"), 0o600); err != nil {
		t.Fatal(err)
	}
}
