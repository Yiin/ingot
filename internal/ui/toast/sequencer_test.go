package toast

import (
	"testing"
	"time"
)

// fakeTimer is one fakeScheduler.after call: fn plus whether cancel was
// called before fire ran it.
type fakeTimer struct {
	d        time.Duration
	fn       func()
	canceled bool
}

// fakeScheduler stands in for glibScheduler so sequencer's replace/reset
// bookkeeping is testable without a running GLib main loop: fire lets a
// test decide exactly when a timer "elapses", and armed/canceled counts
// let a test assert that a replaced toast never leaves two timers live at
// once — the literal shape of "never stack".
type fakeScheduler struct {
	timers   []*fakeTimer
	canceled int
}

func (f *fakeScheduler) after(d time.Duration, fn func()) func() {
	t := &fakeTimer{d: d, fn: fn}
	f.timers = append(f.timers, t)
	idx := len(f.timers) - 1
	return func() {
		if !f.timers[idx].canceled {
			f.timers[idx].canceled = true
			f.canceled++
		}
	}
}

// fire runs timer i's fn if it was never canceled, mirroring what
// glib.TimeoutAdd would do had d actually elapsed.
func (f *fakeScheduler) fire(i int) {
	t := f.timers[i]
	if !t.canceled {
		t.fn()
	}
}

func newCountingSequencer(sched scheduler) (*sequencer, *int, *int, *int) {
	var enters, resets, exits int
	s := newSequencer(sched,
		func() { enters++ },
		func() { resets++ },
		func() { exits++ },
	)
	return s, &enters, &resets, &exits
}

func TestSequencer_FirstShow_EntersAndArmsOneTimer(t *testing.T) {
	sched := &fakeScheduler{}
	s, enters, resets, exits := newCountingSequencer(sched)

	s.show()

	if *enters != 1 || *resets != 0 || *exits != 0 {
		t.Fatalf("enters=%d resets=%d exits=%d, want 1,0,0", *enters, *resets, *exits)
	}
	if len(sched.timers) != 1 {
		t.Fatalf("armed %d timers, want 1", len(sched.timers))
	}
	if sched.timers[0].d != HoldDuration {
		t.Errorf("armed for %v, want HoldDuration %v", sched.timers[0].d, HoldDuration)
	}
	if !s.visible {
		t.Error("visible = false after show(), want true")
	}
}

// TestSequencer_SecondShowWithinHold_ReplacesRatherThanStacks is the
// direct unit-level analogue of the acceptance criterion: "a second
// toast within the hold window replaces rather than stacks."
func TestSequencer_SecondShowWithinHold_ReplacesRatherThanStacks(t *testing.T) {
	sched := &fakeScheduler{}
	s, enters, resets, exits := newCountingSequencer(sched)

	s.show() // timer 0
	s.show() // must cancel timer 0, arm timer 1 — never both live

	if *enters != 1 {
		t.Errorf("enters = %d, want 1 (a replace must not replay the fade-in)", *enters)
	}
	if *resets != 1 {
		t.Errorf("resets = %d, want 1", *resets)
	}
	if sched.canceled != 1 {
		t.Errorf("canceled = %d, want 1 (exactly the first timer)", sched.canceled)
	}
	if len(sched.timers) != 2 {
		t.Fatalf("armed %d timers, want 2", len(sched.timers))
	}

	// The stale first timer, if it were to fire anyway, must not call
	// onExit — that would be the toast hiding itself out from under a
	// still-held replacement.
	sched.fire(0)
	if *exits != 0 {
		t.Fatal("the canceled first timer still fired onExit — the toast would stack/hide early")
	}

	sched.fire(1)
	if *exits != 1 {
		t.Errorf("exits = %d after the live timer fired, want 1", *exits)
	}
	if s.visible {
		t.Error("visible = true after onExit fired, want false")
	}
}

func TestSequencer_ShowAfterHide_IsTreatedAsFresh(t *testing.T) {
	sched := &fakeScheduler{}
	s, enters, _, exits := newCountingSequencer(sched)

	s.show()
	sched.fire(0) // hold elapses, toast hides
	if *exits != 1 {
		t.Fatalf("exits = %d, want 1", *exits)
	}

	s.show()
	if *enters != 2 {
		t.Errorf("enters = %d, want 2 (a show after hide is a fresh toast, not a replace)", *enters)
	}
	if !s.visible {
		t.Error("visible = false after the second fresh show(), want true")
	}
}

func TestSequencer_ThirdShowWithinHold_StillOnlyOneLiveTimer(t *testing.T) {
	sched := &fakeScheduler{}
	s, enters, resets, _ := newCountingSequencer(sched)

	s.show()
	s.show()
	s.show()

	if *enters != 1 {
		t.Errorf("enters = %d, want 1", *enters)
	}
	if *resets != 2 {
		t.Errorf("resets = %d, want 2", *resets)
	}
	if sched.canceled != 2 {
		t.Errorf("canceled = %d, want 2 (every prior timer, each time)", sched.canceled)
	}
	live := 0
	for _, tm := range sched.timers {
		if !tm.canceled {
			live++
		}
	}
	if live != 1 {
		t.Errorf("%d timers left live, want exactly 1", live)
	}
}
