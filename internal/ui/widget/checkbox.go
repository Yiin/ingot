package widget

import (
	"math"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// Checkbox is the panel's 17dp circular checkbox. It is drawn by hand
// (gtk.DrawingArea + cairo), not backed by a real GtkCheckButton: the
// fill-then-tick-sweep flourish (checkProgress, in easing.go) has no
// equivalent in GTK's built-in checkbutton animation.
//
// It is deliberately not focusable — the note row that owns it handles
// keyboard focus and calls SetChecked in response.
type Checkbox struct {
	*gtk.DrawingArea

	checked   bool
	animating bool
	animStart int64 // µs, from the frame clock; -1 until the first tick
	elapsed   time.Duration
	tickID    uint

	toggled []func(checked bool)
}

// NewCheckbox creates an unchecked Checkbox at its natural 17x17dp size.
// A click toggles it and fires ConnectToggled; it takes no keyboard focus.
func NewCheckbox() *Checkbox {
	area := gtk.NewDrawingArea()
	area.SetContentWidth(theme.CheckSize)
	area.SetContentHeight(theme.CheckSize)
	area.SetFocusable(false)
	area.SetCanFocus(false)

	c := &Checkbox{DrawingArea: area}
	area.SetDrawFunc(c.draw)

	click := gtk.NewGestureClick()
	click.ConnectReleased(func(nPress int, x, y float64) {
		c.SetChecked(!c.checked, true)
	})
	area.AddController(click)

	return c
}

// Checked reports the current state, ignoring any in-flight animation.
func (c *Checkbox) Checked() bool { return c.checked }

// SetChecked sets the state. When animate is true and gtk-enable-animations
// is on, it plays the 220ms fill-then-tick animation (or its reverse)
// instead of snapping straight to the target.
func (c *Checkbox) SetChecked(checked bool, animate bool) {
	if checked == c.checked {
		return
	}
	c.checked = checked
	if animate && enableAnimations() {
		c.startAnimation()
	} else {
		c.stopAnimation()
		c.QueueDraw()
	}
	for _, f := range c.toggled {
		f(c.checked)
	}
}

// ConnectToggled registers f to run whenever SetChecked changes the state,
// whether called directly or from a user click. This is a plain Go
// callback list, not a GObject signal: the DrawingArea underneath has no
// "toggled" of its own.
func (c *Checkbox) ConnectToggled(f func(checked bool)) {
	c.toggled = append(c.toggled, f)
}

func (c *Checkbox) startAnimation() {
	c.stopAnimation()
	c.animating = true
	c.animStart = -1
	c.elapsed = 0
	c.tickID = c.AddTickCallback(c.onTick)
}

func (c *Checkbox) stopAnimation() {
	if c.animating {
		c.RemoveTickCallback(c.tickID)
	}
	c.animating = false
}

func (c *Checkbox) onTick(_ gtk.Widgetter, frameClock gdk.FrameClocker) bool {
	now := gdk.BaseFrameClock(frameClock).FrameTime()
	if c.animStart < 0 {
		c.animStart = now
	}
	c.elapsed = time.Duration(now-c.animStart) * time.Microsecond
	c.QueueDraw()
	if c.elapsed >= checkDuration {
		c.elapsed = checkDuration
		c.animating = false
		return false
	}
	return true
}

// progress returns the current (fill, tick) pair in [0,1], run in reverse
// when animating towards unchecked.
func (c *Checkbox) progress() (fill, tick float64) {
	if !c.animating {
		if c.checked {
			return 1, 1
		}
		return 0, 0
	}
	fill, tick = checkProgress(c.elapsed)
	if !c.checked {
		fill, tick = 1-fill, 1-tick
	}
	return fill, tick
}

func (c *Checkbox) draw(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
	fill, tick := c.progress()

	cx, cy := float64(width)/2, float64(height)/2
	radius := (float64(theme.CheckSize) - theme.CheckStroke) / 2

	if fill < 1 {
		rr, rg, rb := hexRGB(theme.CheckRing)
		cr.NewPath()
		cr.Arc(cx, cy, radius, 0, 2*math.Pi)
		cr.SetLineWidth(theme.CheckStroke)
		cr.SetSourceRGBA(rr, rg, rb, 1-fill)
		cr.Stroke()
	}

	if fill > 0 {
		ar, ag, ab := hexRGB(theme.Accent)
		cr.NewPath()
		cr.Arc(cx, cy, radius, 0, 2*math.Pi)
		cr.SetSourceRGBA(ar, ag, ab, fill)
		cr.Fill()
	}

	if tick > 0 {
		drawTick(cr, cx, cy, radius, tick)
	}
}

// drawTick strokes a white checkmark inscribed in the circle of the given
// radius, revealing reveal (0..1) of its length. This is the spec's
// stroke-dashoffset sweep, done with cairo's dash pattern instead: a
// single "on" dash the length of the revealed path, followed by a gap
// longer than the rest of the path so it never wraps back to visible.
func drawTick(cr *cairo.Context, cx, cy, radius, reveal float64) {
	p1x, p1y := cx-radius*0.5, cy+radius*0.02
	p2x, p2y := cx-radius*0.12, cy+radius*0.42
	p3x, p3y := cx+radius*0.55, cy-radius*0.32

	leg1 := math.Hypot(p2x-p1x, p2y-p1y)
	leg2 := math.Hypot(p3x-p2x, p3y-p2y)
	total := leg1 + leg2

	cr.Save()
	defer cr.Restore()

	cr.NewPath()
	cr.MoveTo(p1x, p1y)
	cr.LineTo(p2x, p2y)
	cr.LineTo(p3x, p3y)
	cr.SetDash([]float64{total * reveal, total * 2}, 0)
	cr.SetLineWidth(theme.CheckStroke)
	cr.SetLineCap(cairo.LineCapRound)
	cr.SetLineJoin(cairo.LineJoinRound)
	cr.SetSourceRGB(1, 1, 1)
	cr.Stroke()
}
