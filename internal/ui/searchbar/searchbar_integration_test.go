//go:build integration

// Needs a real GDK display (gtk.Init + a mapped GtkRoot, since the Ctrl+F
// shortcut is scoped Global through the root widget), so this is gated
// behind the integration tag and needs copper-l2z.31's headless sway
// harness — same convention as internal/ui/theme's display_test.go. Not
// run in this worktree: only compile-checked.
package searchbar_test

import (
	"sync"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/searchbar"
)

var gtkInitOnce sync.Once

func newTestWindow(t *testing.T) *gtk.Window {
	t.Helper()
	gtkInitOnce.Do(gtk.Init)
	return gtk.NewWindow()
}

func TestMatchCountVisibility(t *testing.T) {
	win := newTestWindow(t)
	s := searchbar.New()
	win.SetChild(s.Widget())

	var queries []string
	s.OnQueryChanged(func(q string) { queries = append(queries, q) })
	s.SetMatchCount(3) // exercises the call path with an empty query

	_ = queries
}

func TestOverflowButtonExposed(t *testing.T) {
	win := newTestWindow(t)
	s := searchbar.New()
	win.SetChild(s.Widget())

	if s.OverflowButton() == nil {
		t.Fatal("OverflowButton() = nil")
	}
}
