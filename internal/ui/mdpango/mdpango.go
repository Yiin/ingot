// Package mdpango renders note Markdown as Pango markup for GtkLabel.
//
// It walks a goldmark AST by hand instead of using goldmark's HTML renderer
// or a hand-rolled inline scanner: CommonMark's left/right-flanking-delimiter
// rules are the reason "5 * 3 * 2 = 30" and "snake_case_name" must stay
// unchanged, and only a real parser gets that right.
package mdpango

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var mdParser = goldmark.DefaultParser()

// Full renders body with block structure preserved: paragraphs, headings,
// lists, and code blocks are separated by \n.
func Full(body string) string {
	return render(body, false)
}

// Collapsed renders body as a single paragraph with no \n anywhere.
// GtkLabel.SetLines caps lines per paragraph, so any \n in a row's clamped
// label would defeat the 3-line cap.
func Collapsed(body string) string {
	return render(body, true)
}

func render(body string, collapse bool) string {
	source := []byte(body)
	doc := mdParser.Parse(text.NewReader(source))
	return joinBlocks(source, doc, collapse)
}

func blockSep(collapse bool) string {
	if collapse {
		return " "
	}
	return "\n"
}

// joinBlocks renders every child of parent as a block and joins them with
// blockSep, dropping empty blocks (e.g. a ThematicBreak).
func joinBlocks(source []byte, parent ast.Node, collapse bool) string {
	var blocks []string
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		if s := renderBlock(source, c, collapse); s != "" {
			blocks = append(blocks, s)
		}
	}
	return strings.Join(blocks, blockSep(collapse))
}

func renderBlock(source []byte, n ast.Node, collapse bool) string {
	switch n.Kind() {
	case ast.KindParagraph, ast.KindTextBlock:
		var b strings.Builder
		renderInline(&b, source, n, collapse)
		return b.String()

	case ast.KindHeading:
		var b strings.Builder
		renderInline(&b, source, n, collapse)
		if b.Len() == 0 {
			return ""
		}
		return "<span weight=\"bold\">" + b.String() + "</span>"

	case ast.KindFencedCodeBlock, ast.KindCodeBlock:
		raw := rawLines(source, n, collapse)
		if raw == "" {
			return ""
		}
		return "<span font_family=\"monospace\">" + escapeText(raw) + "</span>"

	case ast.KindHTMLBlock:
		hb := n.(*ast.HTMLBlock)
		raw := string(hb.Lines().Value(source))
		if hb.HasClosure() {
			raw += string(hb.ClosureLine.Value(source))
		}
		raw = normalizeBreaks(raw, collapse)
		return escapeText(raw)

	case ast.KindList:
		var items []string
		for item := n.FirstChild(); item != nil; item = item.NextSibling() {
			if body := joinBlocks(source, item, collapse); body != "" {
				items = append(items, "• "+body)
			}
		}
		return strings.Join(items, blockSep(collapse))

	case ast.KindThematicBreak:
		return ""

	default:
		// Safety net: never silently drop content for a block kind we do
		// not special-case (Blockquote, ListItem reached directly, etc).
		return joinBlocks(source, n, collapse)
	}
}

// renderInline walks the inline children of a block node (Paragraph,
// Heading, Emphasis, Link, ...) and writes Pango markup into b.
func renderInline(b *strings.Builder, source []byte, parent ast.Node, collapse bool) {
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		switch n := c.(type) {
		case *ast.Text:
			b.WriteString(escapeText(string(n.Value(source))))
			if n.SoftLineBreak() || n.HardLineBreak() {
				b.WriteString(" ")
			}

		case *ast.String:
			b.WriteString(escapeText(string(n.Value)))

		case *ast.CodeSpan:
			raw := codeSpanText(source, n)
			raw = normalizeBreaks(raw, collapse)
			b.WriteString("<tt>" + escapeText(raw) + "</tt>")

		case *ast.Emphasis:
			open, close := "<i>", "</i>"
			if n.Level >= 2 {
				open, close = "<b>", "</b>"
			}
			b.WriteString(open)
			renderInline(b, source, n, collapse)
			b.WriteString(close)

		case *ast.Link:
			b.WriteString("<a href=\"" + escapeAttr(string(n.Destination)) + "\">")
			renderInline(b, source, n, collapse)
			b.WriteString("</a>")

		case *ast.AutoLink:
			href := escapeAttr(string(n.URL(source)))
			label := escapeText(string(n.Label(source)))
			b.WriteString("<a href=\"" + href + "\">" + label + "</a>")

		case *ast.RawHTML:
			raw := string(n.Segments.Value(source))
			raw = normalizeBreaks(raw, collapse)
			b.WriteString(escapeText(raw))

		default:
			// Safety net for kinds not special-cased (e.g. Image: render
			// its alt text, dropping the destination).
			renderInline(b, source, c, collapse)
		}
	}
}

func codeSpanText(source []byte, n *ast.CodeSpan) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.Write(t.Value(source))
		}
	}
	return b.String()
}

func rawLines(source []byte, n ast.Node, collapse bool) string {
	raw := string(n.Lines().Value(source))
	return normalizeBreaks(raw, collapse)
}

// normalizeBreaks trims a single trailing newline and, when collapsing to a
// single paragraph, turns every remaining \n into a space.
func normalizeBreaks(raw string, collapse bool) string {
	raw = strings.TrimSuffix(raw, "\n")
	if collapse {
		raw = strings.ReplaceAll(raw, "\n", " ")
	}
	return raw
}
