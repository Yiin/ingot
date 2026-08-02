//go:build integration

// IsTextFocused asks GTK which widget actually holds focus, so unlike
// ShouldGateForList's pure policy it needs a real display. Gated behind
// the integration tag and the headless sway harness, same convention as
// internal/ui/notelist/list_integration_test.go. Do not run this against
// a live desktop session: nothing here is ever shown, but it is still a
// real GTK client on whatever WAYLAND_DISPLAY points at.
package keymap

import (
	"sync"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

var gtkInitOnce sync.Once

func requireDisplay(t *testing.T) {
	t.Helper()
	gtkInitOnce.Do(gtk.Init)
	if gdk.DisplayGetDefault() == nil {
		t.Skip("no GDK display available")
	}
}

func pump() {
	ctx := glib.MainContextDefault()
	for ctx.Iteration(false) {
	}
}

// TestIsTextFocusedRecognisesBothTextWidgets pins the exact gap that let
// the gate swallow a keystroke out of a live editor.
//
// IsTextFocused originally matched *gtk.Text only. That is what GtkEntry
// and GtkSearchEntry focus, so the search field was handled — but the
// composer, and the inline row editor built from one, are GtkTextView.
// The inline editor lives inside a note row, which is a descendant of the
// GtkListView the gate's capture-phase controller sits on, so the gate
// saw its keys first, resolved Return as edit-inline and consumed it. An
// inline edit could be opened and never committed.
//
// Both cases are asserted together because passing one while failing the
// other is exactly the shape of that bug.
func TestIsTextFocusedRecognisesBothTextWidgets(t *testing.T) {
	requireDisplay(t)

	tests := []struct {
		name  string
		build func() gtk.Widgetter
		want  bool
	}{
		{
			name:  "GtkText (what GtkEntry and GtkSearchEntry focus)",
			build: func() gtk.Widgetter { return gtk.NewText() },
			want:  true,
		},
		{
			name:  "GtkTextView (the composer and the inline row editor)",
			build: func() gtk.Widgetter { return gtk.NewTextView() },
			want:  true,
		},
		{
			name: "a focusable non-text widget must not gate",
			build: func() gtk.Widgetter {
				b := gtk.NewButtonWithLabel("not text")
				b.SetFocusable(true)
				return b
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			win := gtk.NewWindow()
			defer win.Destroy()

			box := gtk.NewBox(gtk.OrientationVertical, 0)
			// probe is the widget IsTextFocused is called against. It is
			// deliberately not the focused one: the real caller passes the
			// GtkListView while focus sits elsewhere in the same root, so
			// the lookup has to go through the root rather than inspect
			// its own argument.
			probe := gtk.NewLabel("probe")
			subject := tt.build()
			box.Append(probe)
			box.Append(subject)
			win.SetChild(box)
			win.Present()
			pump()

			gtk.BaseWidget(subject).GrabFocus()
			pump()

			if got := IsTextFocused(probe); got != tt.want {
				t.Errorf("IsTextFocused = %v, want %v (focused widget: %T)",
					got, tt.want, win.Focus())
			}
		})
	}
}
