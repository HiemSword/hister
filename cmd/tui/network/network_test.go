// SPDX-FileContributor: 4evy <git@evy.pink>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package network

import "testing"

func TestDecodeResultsPreservesEmptyResultMetadata(t *testing.T) {
	results, err := decodeResults([]byte(`{
		"documents": [],
		"history": [],
		"semantic_hits": [],
		"query_suggestion": "corrected query",
		"search_duration": "0.01 seconds"
	}`))
	if err != nil {
		t.Fatalf("decodeResults() error = %v", err)
	}
	if results.QuerySuggestion != "corrected query" {
		t.Fatalf("query suggestion = %q, want corrected query", results.QuerySuggestion)
	}
	if results.SearchDuration != "0.01 seconds" {
		t.Fatalf("search duration = %q, want 0.01 seconds", results.SearchDuration)
	}
}
