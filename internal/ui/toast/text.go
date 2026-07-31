package toast

import (
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

// maxLabelChars caps a toast's natural width so an unbounded input —
// e.g. a paragraph-sized PRIMARY capture handed to Notifier.Captured —
// can't blow either toast out to a screen-wide or multi-line surface.
// Neither toast is a scrollable or multi-line UI element per the spec.
const maxLabelChars = 60

// newToastLabel returns a GtkLabel configured for a toast's single-line,
// ellipsized, width-capped text — shared by HUD and InPanel so both
// widgets clamp identically.
func newToastLabel() *gtk.Label {
	label := gtk.NewLabel("")
	label.SetSingleLineMode(true)
	label.SetEllipsize(pango.EllipsizeEnd)
	label.SetMaxWidthChars(maxLabelChars)
	return label
}

// newlineCollapser turns any embedded line break into a space before a
// toast label displays it — SetSingleLineMode alone only stops GtkLabel
// from wrapping, it does not stop a literal newline byte from still
// being present in the Pango layout.
var newlineCollapser = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")
