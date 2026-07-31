package notelist

import (
	"strings"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// interCapHeight is Inter Variable's cap height as a fraction of its em
// square (1490/2048, from the font's own metrics table) — the ratio
// capHeightPx uses to place the section rule on the title's cap-height
// midline rather than its full line-box midline, since Pango exposes no
// cap-height query of its own.
const interCapHeight = 1490.0 / 2048.0

// capHeightPx returns fontPx's cap height in device pixels.
func capHeightPx(fontPx float64) float64 { return interCapHeight * fontPx }

// capMidY returns the y coordinate, in the header box's own coordinate
// space, of the section title's cap-height midline, given the label's
// Pango baseline (pango.Layout.Baseline(), already divided by
// pango.SCALE). The header's DrawingArea is valign-Fill within the same
// box as the label, so its origin coincides with the label's line-box
// top — which is what makes the baseline a valid y offset here.
func capMidY(baselinePx float64) float64 {
	return baselinePx - capHeightPx(theme.FontSection)/2
}

// sectionHeader is the recycled widget bound to one GtkListHeader:
// "SECTION TITLE ─────────". The rule is a DrawingArea, not a CSS
// border, because it must land on the label's cap-height midline, which
// only a real Pango layout query (not GTK CSS) can locate.
type sectionHeader struct {
	*gtk.Box

	label *gtk.Label
	rule  *gtk.DrawingArea
}

func newSectionHeader() *sectionHeader {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.AddCSSClass("section-header")

	label := gtk.NewLabel("")
	label.SetXAlign(0)
	label.SetVAlign(gtk.AlignStart)
	label.SetHExpand(false)

	rule := gtk.NewDrawingArea()
	rule.SetHExpand(true)
	rule.SetVAlign(gtk.AlignFill)
	rule.AddCSSClass("section-rule")
	rule.SetCanTarget(false)

	box.Append(label)
	box.Append(rule)

	h := &sectionHeader{Box: box, label: label, rule: rule}
	rule.SetDrawFunc(h.drawRule)
	return h
}

// SetTitle uppercases and sets the header's title text — uppercase is
// done here in Go, deliberately not via font-feature-settings.
//
// An empty title (a project with no declared sections gets one implicit
// unnamed section, per internal/ui/panel's contract) hides the whole
// header: GtkListView's SetSectionSorter still fires exactly one header
// bind for that lone section, but the spec wants "no header and no rule,
// flush under the search field" — a visible-false widget takes no
// layout space at all, margins included.
func (h *sectionHeader) SetTitle(title string) {
	if title == "" {
		h.SetVisible(false)
		return
	}
	h.SetVisible(true)
	h.label.SetText(displayTitle(title))
	h.rule.QueueDraw()
}

// displayTitle is SetTitle's rendering step, split out so the "uppercase
// in Go, not by font feature" contract is unit-testable without a GTK
// display.
func displayTitle(title string) string { return strings.ToUpper(title) }

func (h *sectionHeader) drawRule(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
	baseline := float64(h.label.Layout().Baseline()) / pango.SCALE
	y := capMidY(baseline)
	// Read at draw time so a colour-scheme change repaints in the new
	// palette — see theme.Colors.
	rr, rg, rb, ra := theme.ParseRGBA(theme.Colors().Rule)

	cr.NewPath()
	cr.MoveTo(0, y)
	cr.LineTo(float64(width), y)
	cr.SetLineWidth(1)
	cr.SetSourceRGBA(rr, rg, rb, ra)
	cr.Stroke()
}
