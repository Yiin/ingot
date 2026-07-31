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
	return render(body, false, nil)
}

// Collapsed renders body as a single paragraph with no \n anywhere.
// GtkLabel.SetLines caps lines per paragraph, so any \n in a row's clamped
// label would defeat the 3-line cap.
func Collapsed(body string) string {
	return render(body, true, nil)
}

// HighlightBackground is the Pango background attribute value a search
// match's highlight span uses (internal/ui/search, copper-l2z.28).
const HighlightBackground = "#0A6CFF1F"

// FullHighlighted renders body like Full, additionally wrapping every
// raw-body byte range in ranges (each a [start, end) pair in the same
// coordinates as searchtext's Hit.Ranges/NoteFilter.Ranges) in a
// background span. The wrap happens at the same literal-text emission
// points Full already uses, so it layers over whatever markup that text
// already carries (bold, italic, code) instead of replacing it. ranges
// need not be sorted or disjoint.
//
// Ranges that fall inside a fenced/indented code block are not
// highlighted — a known limitation, code blocks being a rare case for
// this app's short notes — nor are ranges inside content goldmark
// synthesizes with no raw-body counterpart (e.g. a footnote reference).
func FullHighlighted(body string, ranges [][2]int) string {
	return render(body, false, mergeRanges(ranges))
}

// CollapsedHighlighted is Collapsed plus FullHighlighted's highlighting.
func CollapsedHighlighted(body string, ranges [][2]int) string {
	return render(body, true, mergeRanges(ranges))
}

func render(body string, collapse bool, ranges [][2]int) string {
	source := []byte(body)
	doc := mdParser.Parse(text.NewReader(source))
	return joinBlocks(source, doc, collapse, ranges)
}

func blockSep(collapse bool) string {
	if collapse {
		return " "
	}
	return "\n"
}

// joinBlocks renders every child of parent as a block and joins them with
// blockSep, dropping empty blocks (e.g. a ThematicBreak).
func joinBlocks(source []byte, parent ast.Node, collapse bool, ranges [][2]int) string {
	var blocks []string
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		if s := renderBlock(source, c, collapse, ranges); s != "" {
			blocks = append(blocks, s)
		}
	}
	return strings.Join(blocks, blockSep(collapse))
}

func renderBlock(source []byte, n ast.Node, collapse bool, ranges [][2]int) string {
	switch n.Kind() {
	case ast.KindParagraph, ast.KindTextBlock:
		var b strings.Builder
		renderInline(&b, source, n, collapse, ranges)
		return b.String()

	case ast.KindHeading:
		var b strings.Builder
		renderInline(&b, source, n, collapse, ranges)
		if b.Len() == 0 {
			return ""
		}
		return "<span weight=\"bold\">" + b.String() + "</span>"

	case ast.KindFencedCodeBlock, ast.KindCodeBlock:
		// Ranges are never applied inside a code block — see
		// FullHighlighted's doc comment.
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
			if body := joinBlocks(source, item, collapse, ranges); body != "" {
				items = append(items, "• "+body)
			}
		}
		return strings.Join(items, blockSep(collapse))

	case ast.KindThematicBreak:
		return ""

	default:
		// Safety net: never silently drop content for a block kind we do
		// not special-case (Blockquote, ListItem reached directly, etc).
		return joinBlocks(source, n, collapse, ranges)
	}
}

// renderInline walks the inline children of a block node (Paragraph,
// Heading, Emphasis, Link, ...) and writes Pango markup into b. ranges is
// nil for the plain (non-highlighted) render path; every literal
// raw-body text run written here goes through writeHighlighted, which is
// a no-op wrap when ranges is empty.
func renderInline(b *strings.Builder, source []byte, parent ast.Node, collapse bool, ranges [][2]int) {
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		switch n := c.(type) {
		case *ast.Text:
			writeHighlighted(b, source, n.Segment.Start, n.Segment.Stop, ranges)
			if n.SoftLineBreak() || n.HardLineBreak() {
				b.WriteString(" ")
			}

		case *ast.String:
			// Synthetic text with no raw-body counterpart (see
			// searchtext's literalBuilder) — never highlighted.
			b.WriteString(escapeText(string(n.Value)))

		case *ast.CodeSpan:
			b.WriteString("<tt>")
			writeCodeSpan(b, source, n, collapse, ranges)
			b.WriteString("</tt>")

		case *ast.Emphasis:
			open, close := "<i>", "</i>"
			if n.Level >= 2 {
				open, close = "<b>", "</b>"
			}
			b.WriteString(open)
			renderInline(b, source, n, collapse, ranges)
			b.WriteString(close)

		case *ast.Link:
			b.WriteString("<a href=\"" + escapeAttr(string(n.Destination)) + "\">")
			renderInline(b, source, n, collapse, ranges)
			b.WriteString("</a>")

		case *ast.AutoLink:
			// An autolink's label is its own URL, which searchtext never
			// indexes (see its walkNode) — never highlighted.
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
			renderInline(b, source, c, collapse, ranges)
		}
	}
}

// writeCodeSpan writes a code span's content between the caller's <tt>
// tags. When ranges is empty it reproduces codeSpanText's exact behavior
// (concatenate every Text child, then normalizeBreaks the whole thing) —
// the well-tested default path is untouched. An active highlight instead
// writes each child's raw span through writeHighlighted directly, which
// does not collapse an embedded raw newline to a space: a code span that
// both spans multiple raw lines and contains an active search match is
// rare enough for this app's short notes to leave as a known limitation
// rather than complicate the offset math further.
func writeCodeSpan(b *strings.Builder, source []byte, n *ast.CodeSpan, collapse bool, ranges [][2]int) {
	if len(ranges) == 0 {
		raw := codeSpanText(source, n)
		b.WriteString(escapeText(normalizeBreaks(raw, collapse)))
		return
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			writeHighlighted(b, source, t.Segment.Start, t.Segment.Stop, ranges)
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
