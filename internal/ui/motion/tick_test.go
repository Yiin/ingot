package motion

import (
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// fakeTickable is a Go double for tickable — no real GTK widget, since
// constructing one needs a live display (see settings_test.go's own
// comment on this repo's plain-vs-integration test split).
type fakeTickable struct {
	added   int
	removed int
}

func (f *fakeTickable) AddTickCallback(gtk.TickCallback) uint {
	f.added++
	return uint(f.added)
}

func (f *fakeTickable) RemoveTickCallback(id uint) { f.removed++ }

// TestAnimateDisabledIsSynchronousAndInstant is the acceptance criterion
// itself, at this package's own layer: with animations off, Animate must
// produce the end state (step(1)) in the same call, calling done, and
// must never touch AddTickCallback — no frame is ever pumped, so there is
// nothing left to block input.
func TestAnimateDisabledIsSynchronousAndInstant(t *testing.T) {
	defer OverrideEnableAnimations(false)()

	tk := &fakeTickable{}
	var steps []float64
	doneCalled := false

	Animate(tk, 200*time.Millisecond, EaseOut, func(p float64) { steps = append(steps, p) }, func() { doneCalled = true })

	if tk.added != 0 {
		t.Errorf("AddTickCallback called %d times, want 0 (disabled)", tk.added)
	}
	if len(steps) != 1 || steps[0] != 1 {
		t.Errorf("step calls = %v, want exactly [1]", steps)
	}
	if !doneCalled {
		t.Error("done was not called")
	}
}

// TestAnimateZeroDurationIsInstant is the same contract for d<=0
// regardless of the animations setting — a zero-length animation is not
// an animation.
func TestAnimateZeroDurationIsInstant(t *testing.T) {
	defer OverrideEnableAnimations(true)()

	tk := &fakeTickable{}
	var last float64 = -1
	Animate(tk, 0, EaseOut, func(p float64) { last = p }, nil)

	if tk.added != 0 {
		t.Errorf("AddTickCallback called %d times, want 0 (zero duration)", tk.added)
	}
	if last != 1 {
		t.Errorf("step(%v), want step(1)", last)
	}
}

// TestAnimateEnabledRegistersOneTickCallback confirms the enabled path
// registers exactly one AddTickCallback and returns immediately — it does
// not (cannot, without a live frame clock) exercise the per-frame body,
// which needs a real gdk.FrameClocker; that is integration-test
// territory like every other display-dependent behaviour in internal/ui.
func TestAnimateEnabledRegistersOneTickCallback(t *testing.T) {
	defer OverrideEnableAnimations(true)()

	tk := &fakeTickable{}
	stepped := false
	Animate(tk, 200*time.Millisecond, EaseOut, func(float64) { stepped = true }, nil)

	if tk.added != 1 {
		t.Errorf("AddTickCallback called %d times, want 1", tk.added)
	}
	if stepped {
		t.Error("step was called before any frame was pumped")
	}
}

func TestAnimatePanicsOnNilStep(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Animate(..., nil, ...) did not panic")
		}
	}()
	Animate(&fakeTickable{}, time.Second, EaseOut, nil, nil)
}

// TestAnimateCancelRemovesTheTickCallback covers the "restart a
// still-running animation" case: cancel must call RemoveTickCallback
// exactly once, without touching step or done again.
func TestAnimateCancelRemovesTheTickCallback(t *testing.T) {
	defer OverrideEnableAnimations(true)()

	tk := &fakeTickable{}
	cancel := Animate(tk, 200*time.Millisecond, EaseOut, func(float64) {}, nil)
	cancel()

	if tk.removed != 1 {
		t.Errorf("RemoveTickCallback called %d times, want 1", tk.removed)
	}

	cancel() // calling cancel twice must not double-remove
	if tk.removed != 1 {
		t.Errorf("RemoveTickCallback called %d times after a second cancel, want still 1", tk.removed)
	}
}

// TestAnimateDisabledCancelIsANoOp confirms the disabled path's cancel
// (returned after step/done already ran synchronously) never touches
// RemoveTickCallback, since AddTickCallback was never called either.
func TestAnimateDisabledCancelIsANoOp(t *testing.T) {
	defer OverrideEnableAnimations(false)()

	tk := &fakeTickable{}
	cancel := Animate(tk, 200*time.Millisecond, EaseOut, func(float64) {}, nil)
	cancel()

	if tk.removed != 0 {
		t.Errorf("RemoveTickCallback called %d times, want 0 (disabled)", tk.removed)
	}
}
