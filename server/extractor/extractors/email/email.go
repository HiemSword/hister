// SPDX-License-Identifier: AGPL-3.0-or-later

// Package extractor_template is a starting point for implementing a new Hister
// extractor. To use it:
//
//  1. Copy this directory to server/extractor/extractors/<myname>/.
//  2. Rename the directory (remove the leading "_" so Go picks it up).
//  3. Change the package declaration below to match the new directory name.
//  4. Rename TemplateExtractor to something descriptive.
//  5. Update matchURLPrefix (and the Match function) for your target site.
//  6. Implement Extract and Preview.
//  7. Register the extractor in DefaultExtractors in registry.go.
//
// The directory name starts with "_" so the Go toolchain ignores it during
// normal builds. That means this file is never compiled as-is, but it is valid
// Go so editors and linters can still analyse it.
package email

import (
	"strings"

	"github.com/asciimoo/hister/server/extractor/sdk"
	"github.com/asciimoo/hister/server/sanitizer"
)

type EmailExtractor struct {
	sdk.ConfigSupport
}

// Keep this assertion so missing or mismatched SDK methods fail compilation.
var _ sdk.Extractor = (*EmailExtractor)(nil)

// Name returns a short human-readable identifier used in log messages and as
// the YAML config key (lowercased). "MyExtractor" → yaml key "myextractor".
func (e *EmailExtractor) Name() string {
	return "Email"
}

// Description returns a short summary of what this extractor does.
// It is surfaced by the /api/config endpoint.
func (e *EmailExtractor) Description() string {
	return "Renders locally indexed email files (.eml) as HTML for preview."
}

// Capabilities declares which phases this extractor participates in. Set
// Enrich when it only annotates documents and should never select their body.
func (e *EmailExtractor) Capabilities() sdk.Capabilities {
	return sdk.Capabilities{Preview: true}
}

// Match returns true for file:// URLs with an .eml extension.
func (e *EmailExtractor) Match(d *sdk.Document) bool {
	if !strings.HasPrefix(d.URL, "file://") {
		return false
	}
	lower := strings.ToLower(d.URL)
	return strings.HasSuffix(lower, ".eml")
}

// Extract is a no-op: indexing is handled by Indexer.AddEmail.
func (e *EmailExtractor) Extract(d *sdk.Document) sdk.ExtractResult {
	return sdk.ExtractFallback(nil)
}

// Preview sanitizes the rendered HTML stored in doc.HTML.
func (e *EmailExtractor) Preview(d *sdk.Document) sdk.PreviewResult {
	if d.HTML == "" {
		return sdk.PreviewFallback(nil)
	}
	return sdk.Previewed(sdk.PreviewResponse{Content: sanitizer.SanitizeHTML(d.HTML)})
}
