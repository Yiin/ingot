package mdpango

import "strings"

// stripControl removes C0 control characters and DEL from s, keeping \n and
// \t. A NUL byte truncates the whole label at the cgo boundary with no error
// from Pango, so every text fragment must pass through this before it is
// spliced into markup.
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// escapeText strips control characters and escapes the characters that are
// significant in Pango markup text content. Call it on every literal
// fragment before splicing it between tags.
func escapeText(s string) string {
	s = stripControl(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// escapeAttr is escapeText plus quote escaping, for values spliced into an
// attribute such as href="...".
func escapeAttr(s string) string {
	s = stripControl(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
