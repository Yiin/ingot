package toast

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

// glibScheduler is scheduler's real implementation: a re-armable GLib
// main-loop timeout, the same glib.TimeoutAdd/glib.SourceRemove idiom
// internal/ui/notelist's overlay scrollbar uses for its own hold-then-
// fade timer.
type glibScheduler struct{}

func (glibScheduler) after(d time.Duration, fn func()) func() {
	// fired guards against the returned cancel func calling
	// glib.SourceRemove on an ID GLib has already recycled for an
	// unrelated source, which is what a post-fire SourceRemove(src) risks
	// once fn has already run — this is the single-threaded GTK main
	// loop, so no lock is needed to make the two mutually exclusive.
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
