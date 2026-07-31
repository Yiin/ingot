package toast

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// newCheckIcon draws the light toast's "filled black circle with a white
// tick" (measured ~14dp at frame 40.5s). It is hand-drawn like
// widget.Checkbox rather than a symbolic GtkImage, for the same reason:
// no stock icon matches this exact mark. Unlike Checkbox it never
// animates — the toast's icon is always in its fully-revealed state — so
// it needs none of Checkbox's tick-sweep machinery, just Checkbox's tick
// geometry at reveal = 1.
func newCheckIcon() *gtk.DrawingArea {
	area := gtk.NewDrawingArea()
	area.SetContentWidth(theme.ToastIconSize)
	area.SetContentHeight(theme.ToastIconSize)
	area.SetFocusable(false)
	area.SetCanFocus(false)
	area.SetDrawFunc(drawCheckIcon)
	return area
}

func drawCheckIcon(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
	cx, cy := float64(width)/2, float64(height)/2
	radius := float64(theme.ToastIconSize) / 2

	cr.NewPath()
	cr.Arc(cx, cy, radius, 0, 2*math.Pi)
	cr.SetSourceRGB(0, 0, 0)
	cr.Fill()

	// Same three-point tick geometry as widget.Checkbox's drawTick, at
	// full reveal — see that function's comment for how the points were
	// chosen to inscribe a checkmark in the circle.
	p1x, p1y := cx-radius*0.5, cy+radius*0.02
	p2x, p2y := cx-radius*0.12, cy+radius*0.42
	p3x, p3y := cx+radius*0.55, cy-radius*0.32

	cr.NewPath()
	cr.MoveTo(p1x, p1y)
	cr.LineTo(p2x, p2y)
	cr.LineTo(p3x, p3y)
	cr.SetLineWidth(theme.CheckStroke)
	cr.SetLineCap(cairo.LineCapRound)
	cr.SetLineJoin(cairo.LineJoinRound)
	cr.SetSourceRGB(1, 1, 1)
	cr.Stroke()
}
