package motion

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

// classTogglable is any widget with CSS classes — every gotk4 widget type
// satisfies it via its embedded gtk.Widget.
type classTogglable interface {
	AddCSSClass(name string)
	RemoveCSSClass(name string)
}

// Scheduler abstracts a one-shot delayed callback so class-toggle timing
// is testable without a live GLib main loop — the same seam
// internal/ui/toast's own private scheduler interface provides for its
// sequencer.
type Scheduler interface {
	// After schedules fn to run once, after d, and returns a cancel func
	// that prevents fn from running if called before d elapses. Calling
	// cancel after fn has already run is a no-op.
	After(d time.Duration, fn func()) (cancel func())
}

// GLibScheduler is Scheduler's real implementation: a GLib main-loop
// timeout.
type GLibScheduler struct{}

// After implements Scheduler.
func (GLibScheduler) After(d time.Duration, fn func()) (cancel func()) {
	var fired bool
	src := glib.TimeoutAdd(uint(d.Milliseconds()), func() bool {
		fired = true
		fn()
		return false
	})
	return func() {
		if !fired && src != 0 {
			glib.SourceRemove(src)
		}
	}
}

// FlashClass adds class to w, then removes it after d — the shared
// "temporary CSS class" bookkeeping duplicated today across
// internal/ui/notelist's just-inserted row and internal/ui/toast's
// toast-in/toast-out classes. New callers should use this instead of
// hand-rolling another AddCSSClass-then-glib.TimeoutAdd pair.
//
// When gtk-enable-animations is off, FlashClass is a no-op: the class is
// never added, since GTK's own CSS engine has already collapsed whatever
// transition or @keyframes rule that class would have triggered to its
// end state, so there is nothing left for the class's own presence to
// gate — the caller's widget must already reflect its resting layout.
//
// The returned cancel func cancels the pending removal without removing
// the class itself — matching Scheduler.After's own contract — so a
// caller that wants to strip the class immediately (e.g. re-triggering
// before d elapses) must call w.RemoveCSSClass(class) itself before
// re-flashing.
func FlashClass(w classTogglable, class string, d time.Duration, sched Scheduler) (cancel func()) {
	if !EnableAnimations() {
		return func() {}
	}
	w.AddCSSClass(class)
	return sched.After(d, func() {
		w.RemoveCSSClass(class)
	})
}
