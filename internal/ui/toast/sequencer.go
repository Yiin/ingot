package toast

import "time"

// scheduler abstracts a re-armable one-shot timer so sequencer's
// replace/reset semantics are testable without a live GLib main loop —
// glibScheduler (scheduler.go) is the real implementation, fakeScheduler
// (sequencer_test.go) is the test double.
type scheduler interface {
	// after schedules fn to run once, after d. The returned cancel func
	// prevents fn from running if called before d elapses; calling it
	// after fn has already run is a no-op.
	after(d time.Duration, fn func()) (cancel func())
}

// sequencer drives one toast's show/hold/hide lifecycle: FadeInDuration
// in, HoldDuration held, FadeOutDuration out (timing.go) — with a second
// show() while already visible replacing the shown content and resetting
// the hold instead of stacking a second toast. That is the acceptance
// criterion this exists to satisfy: "a second toast within the hold
// window replaces rather than stacks."
//
// sequencer owns no GTK state itself: HUD and InPanel each supply the
// onEnter/onHoldReset/onExit callbacks that do the actual CSS-class or
// reveal-child work. That split makes the part of this actually worth
// testing — the timer replace/cancel bookkeeping — testable without a
// GTK display or a running main loop; see sequencer_test.go.
type sequencer struct {
	sched   scheduler
	cancel  func()
	visible bool

	onEnter     func() // hidden -> visible: play the fade-in and show the surface
	onHoldReset func() // already visible: clear any in-flight fade-out, no fade-in replay
	onExit      func() // hold elapsed: play the fade-out and hide the surface
}

func newSequencer(sched scheduler, onEnter, onHoldReset, onExit func()) *sequencer {
	return &sequencer{sched: sched, onEnter: onEnter, onHoldReset: onHoldReset, onExit: onExit}
}

// show arms (or re-arms) the toast's hold timer. The caller updates the
// shown content (text) before calling show — sequencer only tracks
// visibility and timing, never the content itself.
func (s *sequencer) show() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	if !s.visible {
		s.visible = true
		s.onEnter()
	} else {
		s.onHoldReset()
	}

	s.cancel = s.sched.after(HoldDuration, func() {
		s.cancel = nil
		s.visible = false
		s.onExit()
	})
}
