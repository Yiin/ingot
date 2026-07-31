package clipfmt

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var mdParser = goldmark.DefaultParser()

// HasMarkup reports whether s contains meaningful Markdown syntax —
// anything beyond plain paragraph text — so the clipboard layer can
// decide whether to offer text/markdown alongside text/plain. It parses
// s with the same CommonMark-aware parser as internal/ui/mdpango and
// internal/store/searchtext rather than scanning for stray "*"/"_"
// characters, so "5 * 3 * 2" and "snake_case_name" correctly report no
// markup.
func HasMarkup(s string) bool {
	doc := mdParser.Parse(text.NewReader([]byte(s)))
	return hasMarkupNode(doc)
}

// markupKinds are node kinds that only exist because the source
// contained real Markdown syntax — as opposed to Document, Paragraph,
// TextBlock, and Text, which a plain unformatted string also produces.
var markupKinds = map[ast.NodeKind]bool{
	ast.KindEmphasis:        true,
	ast.KindCodeSpan:        true,
	ast.KindLink:            true,
	ast.KindAutoLink:        true,
	ast.KindImage:           true,
	ast.KindList:            true,
	ast.KindHeading:         true,
	ast.KindBlockquote:      true,
	ast.KindFencedCodeBlock: true,
	ast.KindCodeBlock:       true,
	ast.KindHTMLBlock:       true,
	ast.KindRawHTML:         true,
	ast.KindThematicBreak:   true,
}

func hasMarkupNode(n ast.Node) bool {
	if markupKinds[n.Kind()] {
		return true
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if hasMarkupNode(c) {
			return true
		}
	}
	return false
}
