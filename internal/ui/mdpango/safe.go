package mdpango

import (
	"regexp"

	"github.com/diamondburned/gotk4/pkg/pango"
)

// anchorTag matches an <a ...> open tag or a closing </a>. It is safe against
// a crafted href because escapeAttr always turns a literal '>' in an
// attribute value into &gt;, so the attribute value itself can never contain
// the '>' that ends the match.
var anchorTag = regexp.MustCompile(`</?a\b[^>]*>`)

// Safe renders body like Full, but only returns markup that GTK can actually
// display. GtkLabel strips <a> tags itself before handing the rest to Pango,
// so pango_parse_markup would reject an otherwise-valid label string with
// "Unknown tag 'a'" — validate against the anchor-stripped copy, but return
// the real markup (with <a> intact) so links keep working in the label.
//
// On any validation error, fall back to fully escaped plain text: this can
// never fail to parse, since it contains no tags at all.
func Safe(body string) string {
	return validate(Full(body), body)
}

// SafeCollapsed is Safe for the single-paragraph markup Collapsed produces —
// what a clamped row label needs, since GtkLabel.SetLines caps lines per
// paragraph and a \n in the label would defeat the cap.
func SafeCollapsed(body string) string {
	return validate(Collapsed(body), body)
}

// SafeHighlighted is Safe plus FullHighlighted's search-match highlighting.
func SafeHighlighted(body string, ranges [][2]int) string {
	return validate(FullHighlighted(body, ranges), body)
}

// SafeCollapsedHighlighted is SafeCollapsed plus FullHighlighted's
// search-match highlighting.
func SafeCollapsedHighlighted(body string, ranges [][2]int) string {
	return validate(CollapsedHighlighted(body, ranges), body)
}

func validate(markup, body string) string {
	if _, _, _, err := pango.ParseMarkup(anchorTag.ReplaceAllString(markup, ""), 0); err != nil {
		return escapeText(body)
	}
	return markup
}
