package editorwindow

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// scheduler abstracts a re-armable one-shot timer, the same split
// internal/ui/toast's sequencer uses for its own hold timer: glibScheduler
// below is the real implementation, fakeScheduler (saver_test.go) is the
// test double that makes saver's debounce/flush bookkeeping testable
// without a running GLib main loop.
type scheduler interface {
	// after schedules fn to run once, after d. The returned cancel func
	// prevents fn from running if called before d elapses; calling it
	// after fn has already run is a no-op.
	after(d time.Duration, fn func()) (cancel func())
}

// glibScheduler is scheduler's real implementation.
type glibScheduler struct{}

func (glibScheduler) after(d time.Duration, fn func()) func() {
	// fired guards the returned cancel func against calling
	// glib.SourceRemove on an ID GLib has already recycled for an
	// unrelated source once fn has already run — safe with no lock since
	// this is the single-threaded GTK main loop.
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

// saver debounces persisting the editor's text: scheduleSave (re)arms a
// timer that, once EditorSaveDebounceMs of inactivity has passed, reads
// the buffer via getText and calls onSave if it actually changed. flush
// does the same immediately, canceling any pending timer first — Close
// and the debounce's own firing both go through it, so closing right
// after a keystroke can never race the debounce, and "closing without an
// explicit save persists the text" holds. onSave never fires for a
// no-op change (e.g. the debounce firing after the text already matches
// what a previous flush just saved).
//
// saver owns no GTK state itself — the part actually worth testing here
// is the replace/cancel timer bookkeeping, exactly like
// internal/ui/toast's sequencer; see saver_test.go.
type saver struct {
	sched  scheduler
	cancel func()
	onSave func(text string)

	lastSaved string
}

func newSaver(sched scheduler, initial string, onSave func(text string)) *saver {
	return &saver{sched: sched, onSave: onSave, lastSaved: initial}
}

// scheduleSave (re)arms the debounce timer. getText is called only once
// the timer actually fires, so it always reads the buffer's live
// contents at flush time, not whatever it held when scheduleSave was
// called.
func (s *saver) scheduleSave(getText func() string) {
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = s.sched.after(theme.EditorSaveDebounceMs*time.Millisecond, func() {
		s.cancel = nil
		s.flush(getText())
	})
}

// flush persists text immediately, canceling any pending debounce timer.
// A no-op if text already matches the last value saved (or applied via
// applyExternal).
func (s *saver) flush(text string) {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if text == s.lastSaved {
		return
	}
	s.lastSaved = text
	if s.onSave != nil {
		s.onSave(text)
	}
}

// applyExternal records text as having arrived from outside (the panel
// row was edited elsewhere), without calling onSave — that would loop
// straight back to whatever just pushed this change in. It reports
// whether text actually differs from the last known value, which is the
// caller's own signal for whether it needs to touch the live GTK buffer
// at all.
func (s *saver) applyExternal(text string) bool {
	if text == s.lastSaved {
		return false
	}
	s.lastSaved = text
	return true
}
