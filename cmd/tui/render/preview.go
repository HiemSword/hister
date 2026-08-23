// SPDX-License-Identifier: AGPL-3.0-or-later

package render

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	readabilityrender "codeberg.org/readeck/go-readability/v2/render"
	"golang.org/x/net/html"

	"github.com/asciimoo/hister/cmd/tui/model"
	"github.com/asciimoo/hister/server/document"
)

func ResultDetailsContent(m *model.Model) string {
	url := m.DetailsURL
	if url == "" {
		url = m.GetSelectedURL()
	}
	title := m.DetailsHintTitle
	if m.DetailsPreview != nil && m.DetailsPreview.Title != "" {
		title = m.DetailsPreview.Title
	}
	if title == "" {
		title = url
	}
	title = sanitizeTerminalText(title)
	url = sanitizeTerminalText(url)
	lines := []string{m.Styles.Title.Render(title), m.Styles.URL.Render(url)}

	doc := detailsDocument(m, url)
	meta := map[string]any(nil)
	if m.DetailsPreview != nil {
		meta = m.DetailsPreview.Meta
	}
	facts := previewFacts(m, doc, meta)
	if len(facts) > 0 {
		lines = append(lines, "", m.Styles.Gray.Render(sanitizeTerminalText(strings.Join(facts, "  ·  "))))
	}
	if doc != nil && doc.Label != "" {
		lines = append(lines, m.Styles.SuggTerm.Render(sanitizeTerminalText("label: "+doc.Label)))
	}
	if description := metaString(meta, "description"); description != "" {
		lines = append(lines, "", m.Styles.SecText.UnsetItalic().Render(sanitizeTerminalText(description)))
	}

	lines = append(lines, "", m.Styles.HelpHeader.Render("Content"), "")
	content := ""
	switch {
	case m.DetailsLoading:
		content = "Loading readable preview…"
	case m.DetailsErr != nil:
		content = "Readable preview unavailable: " + m.DetailsErr.Error()
		if doc != nil && strings.TrimSpace(doc.Text) != "" {
			content += "\n\n" + strings.TrimSpace(doc.Text)
		}
	case m.DetailsPreview != nil:
		content = previewPlainText(m.DetailsPreview.Template, m.DetailsPreview.Content)
	case doc != nil:
		content = strings.TrimSpace(doc.Text)
	}
	if content == "" {
		content = "No readable content is available for this result."
	}
	content = sanitizeTerminalText(content)
	lines = append(lines, m.Styles.SecText.UnsetItalic().UnsetFaint().Render(content))
	return strings.Join(lines, "\n")
}

func detailsDocument(m *model.Model, url string) *document.Document {
	for _, doc := range m.VisibleDocuments() {
		if doc != nil && doc.URL == url {
			return doc
		}
	}
	return nil
}

func previewFacts(m *model.Model, doc *document.Document, meta map[string]any) []string {
	var facts []string
	for _, key := range []string{"author", "published", "type", "site_name"} {
		if value := metaString(meta, key); value != "" {
			if key == "published" {
				value = formatPreviewDate(value)
			}
			facts = append(facts, value)
		}
	}
	if len(facts) == 0 && doc != nil {
		if doc.Domain != "" {
			facts = append(facts, doc.Domain)
		}
		if doc.Language != "" {
			facts = append(facts, "language "+doc.Language)
		}
		facts = append(facts, fmt.Sprintf("type %v", doc.Type))
	}
	added := int64(0)
	versionCount := 0
	if m.DetailsPreview != nil {
		added = m.DetailsPreview.Added
		versionCount = m.DetailsPreview.VersionCount
	} else if doc != nil {
		added = doc.Added
	}
	if age := relativeTime(added); age != "" {
		facts = append(facts, "indexed "+age+" ago")
	}
	if versionCount > 0 {
		facts = append(facts, fmt.Sprintf("%d previous version(s)", versionCount))
	}
	return facts
}

func metaString(meta map[string]any, key string) string {
	value, _ := meta[key].(string)
	return strings.TrimSpace(value)
}

func formatPreviewDate(raw string) string {
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.Format("2 Jan 2006")
	}
	return raw
}

func previewPlainText(template, content string) string {
	if template == "video" {
		return videoPreviewText(content)
	}
	if !strings.Contains(content, "<") {
		return strings.TrimSpace(content)
	}
	return htmlPreviewText(content)
}

func videoPreviewText(content string) string {
	var video struct {
		Uploader          string `json:"uploader"`
		DurationFormatted string `json:"durationFormatted"`
		UploadDate        string `json:"uploadDate"`
		ViewCount         int64  `json:"viewCount"`
		Description       string `json:"description"`
		Transcript        string `json:"transcript"`
		Chapters          []struct {
			Title     string `json:"title"`
			StartTime string `json:"startTime"`
		} `json:"chapters"`
	}
	if err := json.Unmarshal([]byte(content), &video); err != nil {
		return strings.TrimSpace(content)
	}
	var sections []string
	var facts []string
	for _, value := range []string{video.Uploader, video.UploadDate, video.DurationFormatted} {
		if value != "" {
			facts = append(facts, value)
		}
	}
	if video.ViewCount > 0 {
		facts = append(facts, fmt.Sprintf("%d views", video.ViewCount))
	}
	if len(facts) > 0 {
		sections = append(sections, strings.Join(facts, " · "))
	}
	if video.Description != "" {
		sections = append(sections, video.Description)
	}
	if len(video.Chapters) > 0 {
		chapters := []string{"Chapters"}
		for _, chapter := range video.Chapters {
			chapters = append(chapters, chapter.StartTime+"  "+chapter.Title)
		}
		sections = append(sections, strings.Join(chapters, "\n"))
	}
	if video.Transcript != "" {
		sections = append(sections, "Transcript\n"+video.Transcript)
	}
	return strings.Join(sections, "\n\n")
}

func htmlPreviewText(content string) string {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return strings.TrimSpace(content)
	}
	return strings.TrimSpace(readabilityrender.InnerText(doc))
}
