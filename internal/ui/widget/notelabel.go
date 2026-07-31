package widget

import (
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"

	"github.com/Yiin/ingot/internal/ui/mdpango"
)

// Label is a GtkLabel pre-configured with the panel's 3-line clamp. All
// five calls in NewLabel are required together — dropping SetWrap or
// SetMaxWidthChars silently defeats SetLines: see the child issue's
// measured failure mode.
type Label struct {
	*gtk.Label

	maxLines int
	body     string
}

// NewLabel returns a Label clamped to maxLines.
func NewLabel(maxLines int) *Label {
	l := gtk.NewLabel("")
	l.SetUseMarkup(true)
	l.SetXAlign(0)
	l.SetVAlign(gtk.AlignStart)
	l.SetWrap(true) // without this, SetLines does nothing
	l.SetWrapMode(pango.WrapWordChar)
	l.SetEllipsize(pango.EllipsizeEnd)
	l.SetLines(maxLines)
	l.SetMaxWidthChars(1) // clamps NATURAL width so the panel width wins

	return &Label{Label: l, maxLines: maxLines}
}

// SetBody renders body as the row's clamped markup.
func (l *Label) SetBody(body string) {
	l.body = body
	l.SetMarkup(clampedMarkup(body))
}

// clampedMarkup is SetBody's rendering step, split out so it is
// unit-testable without a GTK display: trim trailing whitespace (a
// captured selection ending in a blank line must not cost the card its
// bottom line), collapse the Markdown to one Pango paragraph — SetLines
// caps lines per paragraph, so any \n would defeat the clamp — and
// validate the result the same way mdpango.Safe validates Full's output.
func clampedMarkup(body string) string {
	trimmed := strings.TrimRight(body, " \t\n\r")
	return mdpango.SafeCollapsed(trimmed)
}

// IsTruncated reports whether the label's current layout actually
// ellipsized. It is only meaningful after allocation, so call it from an
// event handler (e.g. a right-click), never from a list factory bind.
func (l *Label) IsTruncated() bool {
	return l.Layout().IsEllipsized()
}

// Expand lifts the line cap so the full body renders, re-rendering through
// mdpango.Full/mdpango.Safe — the block structure the collapsed clamp
// deliberately throws away.
func (l *Label) Expand() {
	l.SetLines(-1)
	l.SetEllipsize(pango.EllipsizeNone)
	l.SetMarkup(mdpango.Safe(l.body))
}

// Collapse reverses Expand, restoring the maxLines clamp.
func (l *Label) Collapse() {
	l.SetLines(l.maxLines)
	l.SetEllipsize(pango.EllipsizeEnd)
	l.SetMarkup(clampedMarkup(l.body))
}
