package motion

import (
	"testing"
	"time"
)

// fakeClassWidget is a Go double for classTogglable.
type fakeClassWidget struct {
	classes map[string]bool
}

func newFakeClassWidget() *fakeClassWidget { return &fakeClassWidget{classes: map[string]bool{}} }

func (f *fakeClassWidget) AddCSSClass(name string)    { f.classes[name] = true }
func (f *fakeClassWidget) RemoveCSSClass(name string) { delete(f.classes, name) }

// fakeScheduler is a Go double for Scheduler: it records the scheduled
// delay/fn instead of touching a real GLib main loop, and lets a test
// fire or cancel it deterministically.
type fakeScheduler struct {
	delay     time.Duration
	fn        func()
	cancelled bool
	calls     int
}

func (f *fakeScheduler) After(d time.Duration, fn func()) (cancel func()) {
	f.calls++
	f.delay = d
	f.fn = fn
	return func() { f.cancelled = true }
}

func (f *fakeScheduler) fire() {
	if !f.cancelled && f.fn != nil {
		f.fn()
	}
}

func TestFlashClassEnabledAddsThenSchedulesRemoval(t *testing.T) {
	defer OverrideEnableAnimations(true)()

	w := newFakeClassWidget()
	sched := &fakeScheduler{}
	cancel := FlashClass(w, "just-inserted", RowInsertDuration, sched)

	if !w.classes["just-inserted"] {
		t.Fatal("FlashClass did not add the class")
	}
	if sched.calls != 1 || sched.delay != RowInsertDuration {
		t.Fatalf("scheduled call = %d after %v, want 1 after %v", sched.calls, sched.delay, RowInsertDuration)
	}

	sched.fire()
	if w.classes["just-inserted"] {
		t.Error("class still present after the scheduled removal fired")
	}

	cancel() // firing after the timer already ran must not panic or misbehave
}

func TestFlashClassEnabledCancelPreventsRemoval(t *testing.T) {
	defer OverrideEnableAnimations(true)()

	w := newFakeClassWidget()
	sched := &fakeScheduler{}
	cancel := FlashClass(w, "duplicate-flash", 300*time.Millisecond, sched)
	cancel()

	if sched.cancelled != true {
		t.Fatal("cancel() did not reach the scheduler")
	}
}

// TestFlashClassDisabledIsANoOp is the acceptance criterion at this
// package's own layer: with animations off, the class must never appear
// at all — GTK has already collapsed whatever CSS rule it would gate to
// its resting end state, so a caller's very next paint already shows the
// final layout.
func TestFlashClassDisabledIsANoOp(t *testing.T) {
	defer OverrideEnableAnimations(false)()

	w := newFakeClassWidget()
	sched := &fakeScheduler{}
	FlashClass(w, "just-inserted", RowInsertDuration, sched)()

	if len(w.classes) != 0 {
		t.Errorf("classes = %v, want none added", w.classes)
	}
	if sched.calls != 0 {
		t.Errorf("scheduler was called %d times, want 0 (disabled)", sched.calls)
	}
}
