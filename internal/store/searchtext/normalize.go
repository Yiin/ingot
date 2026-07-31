package searchtext

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"golang.org/x/text/unicode/norm"
)

var mdParser = goldmark.DefaultParser()

// Normalized is text normalized for search: NFKD-decomposed, combining
// marks stripped, case-folded, and — when computed via Normalize — with
// Markdown syntax stripped down to literal content. It retains a
// byte-offset map back into the raw text it was computed from, so a
// match found in Text can be translated back into raw-body byte ranges.
type Normalized struct {
	// Text is the normalized form.
	Text string
	// starts[i]/ends[i] are the raw-text [start, end) byte span that
	// produced byte i of Text, for every i in [0, len(Text)). A raw
	// character that folds to more than one output byte repeats its
	// span across each of them; the pair is what lets a match's [start,
	// end) in Text become starts[start], ends[end-1] in raw bytes even
	// when folding or Markdown stripping changed the byte count in
	// between.
	starts, ends []int
}

// Normalize computes body's search-normalized form: a Markdown parse
// extracts literal text (dropping emphasis markers, code-span
// backticks, and link syntax — but keeping link text and code-span
// content), which is then NFKD-decomposed, stripped of combining
// marks, and case-folded.
//
// This is comparatively expensive (a Markdown parse plus a rune-by-rune
// pass); a caller that calls it repeatedly for the same body — as
// fsstore's Search does, once per note per keystroke — should cache the
// result and only recompute when the body changes.
func Normalize(body string) Normalized {
	lit, litOffsets := extractLiteral(body)
	return foldRunes(lit, litOffsets)
}

// extractLiteral walks body's Markdown AST and returns its literal text
// content — the same content mdpango would render, minus all markup —
// together with a per-byte map back into body's raw offsets.
func extractLiteral(body string) (string, []int) {
	source := []byte(body)
	doc := mdParser.Parse(text.NewReader(source))
	b := &literalBuilder{}
	b.walkChildren(source, doc)
	return string(b.text), b.offsets
}

// literalBuilder accumulates literal text extracted from a Markdown AST
// alongside a parallel per-byte raw-offset slice.
type literalBuilder struct {
	text    []byte
	offsets []int
	// lastOff is the raw offset just past the most recently written real
	// (non-synthetic) content, used as the anchor offset for synthetic
	// separators inserted between blocks and soft/hard line breaks.
	lastOff int
	// pendingSep is set when a block boundary or line break has been
	// seen but not yet followed by more content. Deferring the write
	// this way — instead of writing the separator immediately — means a
	// trailing block never leaves a trailing space in Text.
	pendingSep bool
}

// markSep records that a separator is owed before the next real
// content, if any ever arrives.
func (b *literalBuilder) markSep() {
	if len(b.text) > 0 {
		b.pendingSep = true
	}
}

func (b *literalBuilder) flushSep() {
	if b.pendingSep {
		b.text = append(b.text, ' ')
		b.offsets = append(b.offsets, b.lastOff)
		b.pendingSep = false
	}
}

func (b *literalBuilder) writeRaw(source []byte, start, stop int) {
	if stop <= start {
		return
	}
	b.flushSep()
	b.text = append(b.text, source[start:stop]...)
	for i := start; i < stop; i++ {
		b.offsets = append(b.offsets, i)
	}
	b.lastOff = stop
}

func (b *literalBuilder) walkChildren(source []byte, parent ast.Node) {
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		b.walkNode(source, c)
	}
}

func (b *literalBuilder) walkNode(source []byte, n ast.Node) {
	switch v := n.(type) {
	case *ast.Text:
		b.writeRaw(source, v.Segment.Start, v.Segment.Stop)
		if v.SoftLineBreak() || v.HardLineBreak() {
			b.markSep()
		}

	case *ast.String:
		// A handful of goldmark constructs synthesize text with no raw
		// counterpart; anchor it to whatever real content precedes it.
		s := string(v.Value)
		if s == "" {
			return
		}
		b.flushSep()
		b.text = append(b.text, s...)
		for range []byte(s) {
			b.offsets = append(b.offsets, b.lastOff)
		}

	case *ast.CodeSpan:
		// Backticks are structural (consumed by the parser, never
		// emitted as Text), so walking the code span's own Text
		// children strips them for free while keeping the content.
		for c := v.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				b.writeRaw(source, t.Segment.Start, t.Segment.Stop)
			}
		}

	case *ast.FencedCodeBlock:
		b.writeLines(source, v.Lines())
		b.markSep()

	case *ast.CodeBlock:
		b.writeLines(source, v.Lines())
		b.markSep()

	case *ast.HTMLBlock, *ast.RawHTML:
		// Raw HTML carries no useful search text and no clean literal
		// content to extract; drop it like any other markup.

	case *ast.ThematicBreak, *ast.AutoLink:
		// No literal text: a thematic break has none, and an autolink's
		// label is its own URL, symmetric with dropping a Link's
		// destination.

	default:
		// Covers Document, Paragraph, TextBlock, Heading, List,
		// ListItem, Blockquote, Emphasis, Link, Image, and any future
		// kind: descend for literal text, and — for block-level
		// containers only — separate this block's content from its
		// neighbor's.
		b.walkChildren(source, n)
		if n.Type() == ast.TypeBlock {
			b.markSep()
		}
	}
}

func (b *literalBuilder) writeLines(source []byte, lines *text.Segments) {
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.writeRaw(source, seg.Start, seg.Stop)
	}
}

// foldRune decomposes r (NFKD), drops combining marks, and lower-cases
// what remains, emitting zero or more folded runes to emit. A single
// accented letter typically folds to exactly one rune — its base
// letter — but decomposition can occasionally yield more than one
// surviving rune.
func foldRune(r rune, emit func(rune)) {
	for _, dr := range norm.NFKD.String(string(r)) {
		if unicode.Is(unicode.Mn, dr) {
			continue
		}
		emit(unicode.ToLower(dr))
	}
}

// foldRunes applies foldRune across lit, building the final Normalized
// text and its raw-span map from lit's own per-byte offsets. Every raw
// rune in lit is a straight byte copy from the source (see writeRaw), so
// its raw span is [litOffsets[bytePos], litOffsets[bytePos] + size),
// where size is however many bytes of lit this decode actually
// consumed — no separate end-offset table needs to travel through
// extractLiteral.
//
// size must come from DecodeRuneInString, not utf8.RuneLen(r): for an
// invalid byte sequence, decoding yields r == utf8.RuneError but
// consumes exactly 1 raw byte, whereas utf8.RuneLen(utf8.RuneError) is
// 3 (RuneError's own valid encoding length) — using RuneLen there would
// walk starts/ends past the actual raw content and hand a caller (e.g.
// Hit.Ranges) an out-of-bounds span.
func foldRunes(lit string, litOffsets []int) Normalized {
	var out strings.Builder
	var starts, ends []int
	for bytePos := 0; bytePos < len(lit); {
		r, size := utf8.DecodeRuneInString(lit[bytePos:])
		rawStart := litOffsets[bytePos]
		rawEnd := rawStart + size
		foldRune(r, func(fr rune) {
			before := out.Len()
			out.WriteRune(fr)
			for i := 0; i < out.Len()-before; i++ {
				starts = append(starts, rawStart)
				ends = append(ends, rawEnd)
			}
		})
		bytePos += size
	}
	return Normalized{Text: out.String(), starts: starts, ends: ends}
}

// foldPlain applies foldRune across s with no offset tracking and no
// Markdown parse — used for search query tokens, which are plain text,
// not Markdown source.
func foldPlain(s string) string {
	var out strings.Builder
	for _, r := range s {
		foldRune(r, func(fr rune) { out.WriteRune(fr) })
	}
	return out.String()
}
