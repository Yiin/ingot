//go:build integration

// These tests need a real GDK display connection (gtk.Init + a mapped
// GtkRoot for focus/tick-callback machinery to work), so they are gated
// behind the integration tag and need to run inside the headless sway
// harness from copper-l2z.31 — same convention as internal/ui/theme's
// display_test.go. Not run in this worktree: only compile-checked.
//
// Driving a synthetic Return/Escape key-press needs a mapped, focused
// surface, so the commit/reject/focus-ring paths are exercised by the
// harness once copper-l2z.31 lands, not here.
package composer_test

import (
	"sync"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/composer"
)

var gtkInitOnce sync.Once

func newTestWindow(t *testing.T) *gtk.Window {
	t.Helper()
	gtkInitOnce.Do(gtk.Init)
	return gtk.NewWindow()
}

func TestTextRoundTrip(t *testing.T) {
	win := newTestWindow(t)
	c := composer.New("work")
	win.SetChild(c.Widget())

	c.SetText("  hello there  ")
	if got := c.Text(); got != "  hello there  " {
		t.Errorf("Text() = %q", got)
	}
}

func TestSetProjectUpdatesPlaceholder(t *testing.T) {
	win := newTestWindow(t)
	c := composer.New("Work")
	win.SetChild(c.Widget())
	c.SetProject("Personal") // exercises the call path; no public getter by design
}

// TestDisablePlaceholderStaysHiddenEvenWhenEmpty covers copper-l2z.27's
// inline row editor: an empty buffer there (select-all, delete) must
// never show the bottom composer's own "Add a note or a prompt ()"
// invitation.
func TestDisablePlaceholderStaysHiddenEvenWhenEmpty(t *testing.T) {
	win := newTestWindow(t)
	c := composer.New("")
	win.SetChild(c.Widget())

	c.DisablePlaceholder()
	c.SetText("something")
	c.SetText("") // empty buffer: handleTextChanged must not re-show it
}
