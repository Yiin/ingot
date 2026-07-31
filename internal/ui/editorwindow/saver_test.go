package editorwindow

import (
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// fakeTimer is one fakeScheduler.after call: fn plus whether cancel was
// called before fire ran it.
type fakeTimer struct {
	d        time.Duration
	fn       func()
	canceled bool
}

// fakeScheduler stands in for glibScheduler so saver's debounce/flush
// bookkeeping is testable without a running GLib main loop — the same
// idiom as internal/ui/toast's own sequencer_test.go.
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

func newCountingSaver(sched scheduler, initial string) (*saver, *[]string) {
	var saved []string
	s := newSaver(sched, initial, func(text string) { saved = append(saved, text) })
	return s, &saved
}

// TestScheduleSave_ArmsOneTimerAtTheDebounceDuration covers "a keystroke
// in the editor updates the panel row within one debounce interval."
func TestScheduleSave_ArmsOneTimerAtTheDebounceDuration(t *testing.T) {
	sched := &fakeScheduler{}
	s, saved := newCountingSaver(sched, "hello")

	s.scheduleSave(func() string { return "hello world" })

	if len(sched.timers) != 1 {
		t.Fatalf("armed %d timers, want 1", len(sched.timers))
	}
	if want := theme.EditorSaveDebounceMs * time.Millisecond; sched.timers[0].d != want {
		t.Errorf("armed for %v, want %v", sched.timers[0].d, want)
	}
	if len(*saved) != 0 {
		t.Fatal("onSave fired before the timer elapsed")
	}

	sched.fire(0)
	if got := *saved; len(got) != 1 || got[0] != "hello world" {
		t.Errorf("saved = %v, want [\"hello world\"]", got)
	}
}

// TestScheduleSave_RapidKeystrokesDebounceToOneTimer is the direct
// analogue of internal/ui/toast's "second show replaces rather than
// stacks": a second scheduleSave before the first fires must cancel it
// and arm a fresh one, never leaving two timers live.
func TestScheduleSave_RapidKeystrokesDebounceToOneTimer(t *testing.T) {
	sched := &fakeScheduler{}
	s, saved := newCountingSaver(sched, "")

	s.scheduleSave(func() string { return "h" })
	s.scheduleSave(func() string { return "he" })
	s.scheduleSave(func() string { return "hel" })

	if len(sched.timers) != 3 {
		t.Fatalf("armed %d timers, want 3", len(sched.timers))
	}
	if sched.canceled != 2 {
		t.Errorf("canceled = %d, want 2 (every prior timer)", sched.canceled)
	}

	// The two stale, canceled timers must not save anything if fired
	// anyway.
	sched.fire(0)
	sched.fire(1)
	if len(*saved) != 0 {
		t.Fatalf("saved = %v after firing canceled timers, want none", *saved)
	}

	sched.fire(2)
	if got := *saved; len(got) != 1 || got[0] != "hel" {
		t.Errorf("saved = %v, want [\"hel\"] (only the live timer's text)", got)
	}
}

// TestFlush_CancelsPendingTimer covers "closing without an explicit
// save persists the text": flush must save immediately and the
// debounce timer it canceled must never fire a second, stale save.
func TestFlush_CancelsPendingTimer(t *testing.T) {
	sched := &fakeScheduler{}
	s, saved := newCountingSaver(sched, "hello")

	s.scheduleSave(func() string { return "hello world" })
	s.flush("hello world!!")

	if got := *saved; len(got) != 1 || got[0] != "hello world!!" {
		t.Errorf("saved = %v, want [\"hello world!!\"]", got)
	}
	if sched.canceled != 1 {
		t.Errorf("canceled = %d, want 1", sched.canceled)
	}

	sched.fire(0) // the canceled debounce timer, if it fired anyway
	if len(*saved) != 1 {
		t.Errorf("saved = %v, want still just the one flush (stale timer must not re-save)", *saved)
	}
}

// TestFlush_NoOpWhenTextUnchanged covers onSave never firing for a
// no-op change — a flush (from Close, say) with nothing new to persist.
func TestFlush_NoOpWhenTextUnchanged(t *testing.T) {
	sched := &fakeScheduler{}
	s, saved := newCountingSaver(sched, "hello")

	s.flush("hello")

	if len(*saved) != 0 {
		t.Errorf("saved = %v, want none (text unchanged)", *saved)
	}
}

// TestApplyExternal_SkipsOwnSaveAndReportsChange covers the feedback-
// loop guard: applying the exact text this saver just saved must report
// "no change" (so the caller never re-writes the live GTK buffer for
// nothing), while a genuinely different external change reports true
// and updates what a later flush compares against.
func TestApplyExternal_SkipsOwnSaveAndReportsChange(t *testing.T) {
	sched := &fakeScheduler{}
	s, saved := newCountingSaver(sched, "hello")

	s.flush("edited by user")
	if changed := s.applyExternal("edited by user"); changed {
		t.Error("applyExternal(the text this saver just saved) = true, want false")
	}

	if changed := s.applyExternal("edited elsewhere"); !changed {
		t.Error("applyExternal(a genuinely new external text) = false, want true")
	}

	// A flush of that same external text afterwards must be a no-op —
	// applyExternal already recorded it as the current lastSaved value.
	s.flush("edited elsewhere")
	if got := *saved; len(got) != 1 || got[0] != "edited by user" {
		t.Errorf("saved = %v, want only the original flush", got)
	}
}
