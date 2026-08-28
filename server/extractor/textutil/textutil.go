// Package textutil provides shared helpers for flattening HTML to plain text
// while keeping the block structure of the original markup.
package textutil

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// textBlockElements are the elements whose boundaries become line breaks when
// markup is flattened to text.
var textBlockElements = map[string]bool{
	"address": true, "article": true, "blockquote": true, "dd": true, "div": true,
	"dl": true, "dt": true, "figcaption": true, "figure": true, "footer": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"header": true, "li": true, "main": true, "ol": true, "p": true, "pre": true,
	"section": true, "table": true, "td": true, "th": true, "tr": true, "ul": true,
}

// SelectionText flattens a selection to plain text. Unlike goquery's Text(),
// which concatenates text nodes with no separator and therefore runs the
// paragraphs of multi-paragraph content together, block elements and <br>
// become line breaks so the shape of the original content survives.
func SelectionText(selection *goquery.Selection) string {
	if selection == nil || selection.Length() == 0 {
		return ""
	}
	var b strings.Builder
	for _, node := range selection.Nodes {
		writeNodeText(&b, node)
	}
	return normalizeText(b.String())
}

func writeNodeText(b *strings.Builder, node *html.Node) {
	if node.Type == html.TextNode {
		b.WriteString(node.Data)
		return
	}
	if node.Type == html.ElementNode {
		name := strings.ToLower(node.Data)
		if name == "script" || name == "style" || name == "svg" || name == "button" {
			return
		}
		if name == "br" {
			writeTextBreak(b)
			return
		}
		if textBlockElements[name] {
			writeTextBreak(b)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			writeNodeText(b, child)
		}
		if textBlockElements[name] {
			writeTextBreak(b)
		}
		return
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		writeNodeText(b, child)
	}
}

func writeTextBreak(b *strings.Builder) {
	if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
		b.WriteByte('\n')
	}
}

// normalizeText collapses runs of whitespace within lines and runs of blank
// lines down to one, and trims the result.
func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			if len(cleaned) > 0 && !blank {
				cleaned = append(cleaned, "")
				blank = true
			}
			continue
		}
		cleaned = append(cleaned, line)
		blank = false
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
