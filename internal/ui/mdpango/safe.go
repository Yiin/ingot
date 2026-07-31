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
	markup := Full(body)
	if _, _, _, err := pango.ParseMarkup(anchorTag.ReplaceAllString(markup, ""), 0); err != nil {
		return escapeText(body)
	}
	return markup
}
