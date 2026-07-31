package motion

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// tickable is any widget gotk4 lets drive a frame-clock animation on —
// every widget type satisfies it via its embedded gtk.Widget, so a
// caller passes its own concrete widget (a *gtk.DrawingArea, a
// *gtk.TextView, ...) with no wrapping needed.
type tickable interface {
	AddTickCallback(func(gtk.Widgetter, gdk.FrameClocker) bool) uint
	RemoveTickCallback(id uint)
}

// Animate drives progress from 0 to 1 over d on w's frame clock, calling
// step(ease(progress)) every frame and done once progress reaches 1 —
// the shared version of the AddTickCallback loop internal/ui/widget's
// checkbox/strikethrough and internal/ui/composer's height growth each
// hand-roll independently today. New AddTickCallback-driven animations
// should use this instead of writing a fourth copy.
//
// When gtk-enable-animations is off (EnableAnimations() == false) or d
// is zero or negative, Animate never touches AddTickCallback at all: it
// calls step(1) and then done, synchronously, in the same call — "every
// duration above becomes 0" for any hand-rolled animation, matching what
// GTK's own CSS engine already does for free for every CSS-driven one.
//
// The gotk4 frame-time gotcha: a frame clock's time is read via
// gdk.BaseFrameClock(clock).FrameTime(), not clock.FrameTime() — the
// latter is not exposed on the gdk.FrameClocker interface itself.
//
// The returned cancel func stops the animation before it reaches 1
// without calling step or done again — the same "restart a still-running
// growth animation" case internal/ui/composer's animateHeightTo already
// handles by hand (RemoveTickCallback-then-restart on every keystroke).
// Calling cancel after the animation already finished on its own is a
// no-op.
func Animate(w tickable, d time.Duration, ease Easing, step func(progress float64), done func()) (cancel func()) {
	if step == nil {
		panic("motion: Animate called with a nil step func")
	}
	if !EnableAnimations() || d <= 0 {
		step(1)
		if done != nil {
			done()
		}
		return func() {}
	}

	var (
		start int64 = -1
		id    uint
		live  = true
	)
	// Returning false from the tick callback itself already unregisters
	// it — matching internal/ui/widget's checkbox.onTick precedent, this
	// never also calls RemoveTickCallback from inside the callback, which
	// would be a redundant, and on some gotk4 versions unsafe, double
	// removal of an id GTK has already recycled. cancel below uses the
	// same live guard for the same reason.
	id = w.AddTickCallback(func(_ gtk.Widgetter, frameClock gdk.FrameClocker) bool {
		now := gdk.BaseFrameClock(frameClock).FrameTime()
		if start < 0 {
			start = now
		}
		elapsed := time.Duration(now-start) * time.Microsecond
		if elapsed >= d {
			step(1)
			live = false
			if done != nil {
				done()
			}
			return false
		}
		step(ease(clamp01(float64(elapsed) / float64(d))))
		return true
	})
	return func() {
		if live {
			live = false
			w.RemoveTickCallback(id)
		}
	}
}
