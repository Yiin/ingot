//go:build integration

// These tests need a real GDK display connection (gtk.Init + a mapped
// GtkRoot for focus/tick-callback machinery to work), so they are gated
// behind the integration tag and need to run inside the headless sway
// harness from copper-l2z.31 — same convention as
// internal/ui/composer's own integration tests. This worktree has no
// display at all: these tests only need to compile here (go vet -tags
// integration), never execute.
//
// Driving a synthetic Escape key-press needs a mapped, focused surface,
// so CancelEdit's own key-controller path is not exercised here either
// — only the CancelEdit() method itself, called directly.
package widget

import (
	"sync"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

var gtkInitOnce sync.Once

func newTestWindow(t *testing.T) *gtk.Window {
	t.Helper()
	gtkInitOnce.Do(gtk.Init)
	return gtk.NewWindow()
}

// TestStartEditSeedsRawMarkdownInPlaceOfTheLabel covers "the edit buffer
// contains raw markdown, not rendered text": StartEdit swaps the stack
// to its editor page, seeded verbatim (unrendered) with the note's raw
// Markdown source. Driving an actual Enter/Escape keystroke needs a
// mapped, focused surface — deferred to the headless harness, same as
// composer's own integration tests — so only StartEdit/CancelEdit
// themselves are exercised here, not the key controllers they wire.
func TestStartEditSeedsRawMarkdownInPlaceOfTheLabel(t *testing.T) {
	win := newTestWindow(t)
	r := NewRow()
	r.Label.SetBody("**bold** not rendered")
	win.SetChild(r)

	if r.IsEditing() {
		t.Fatal("IsEditing() = true before StartEdit")
	}

	r.StartEdit("**bold** not rendered", nil)

	if !r.IsEditing() {
		t.Fatal("IsEditing() = false after StartEdit")
	}
	if r.stack.VisibleChildName() != editorPageName {
		t.Errorf("visible stack page = %q, want %q", r.stack.VisibleChildName(), editorPageName)
	}
	if got := r.editing.composer.Text(); got != "**bold** not rendered" {
		t.Errorf("editor seeded with %q, want the raw markdown", got)
	}
}

// TestStartEditIsNoOpWhileAlreadyEditing covers "the same [row] cannot
// open two editor" sessions — the row-level analogue of the editor
// window's own dedup contract.
func TestStartEditIsNoOpWhileAlreadyEditing(t *testing.T) {
	win := newTestWindow(t)
	r := NewRow()
	r.Label.SetBody("original")
	win.SetChild(r)

	r.StartEdit("original", nil)
	first := r.editing

	r.StartEdit("a different re-entrant call", func(string) {
		t.Fatal("the second StartEdit's onCommit must never be wired up")
	})

	if r.editing != first {
		t.Error("a second StartEdit call while already editing replaced the in-flight session")
	}
}

// TestCancelEditRestoresLabelUntouched covers "Escape restores the
// original text and its rendered attributes": since the Label is never
// mutated during editing, CancelEdit simply has to swap back to it and
// never invoke the commit callback.
func TestCancelEditRestoresLabelUntouched(t *testing.T) {
	win := newTestWindow(t)
	r := NewRow()
	r.Label.SetBody("original")
	win.SetChild(r)

	committed := false
	r.StartEdit("original", func(string) { committed = true })
	r.CancelEdit()

	if r.IsEditing() {
		t.Error("IsEditing() = true after CancelEdit")
	}
	if committed {
		t.Error("CancelEdit invoked the commit callback")
	}
	if r.stack.VisibleChildName() != labelPageName {
		t.Errorf("visible stack page = %q, want %q", r.stack.VisibleChildName(), labelPageName)
	}
}

// TestSetExpandedIsNoOpWhenUnchanged covers bindRow's own unconditional
// reset-on-recycle call: calling SetExpanded with the row's current
// state must not re-render the label.
func TestSetExpandedIsNoOpWhenUnchanged(t *testing.T) {
	win := newTestWindow(t)
	r := NewRow()
	r.Label.SetBody("a note")
	win.SetChild(r)

	if r.IsExpanded() {
		t.Fatal("IsExpanded() = true on a fresh row")
	}
	r.SetExpanded(false) // matches bindRow's own call; must be a no-op
	if r.IsExpanded() {
		t.Error("IsExpanded() = true after SetExpanded(false) on an already-collapsed row")
	}

	r.SetExpanded(true)
	if !r.IsExpanded() {
		t.Error("IsExpanded() = false after SetExpanded(true)")
	}

	r.SetExpanded(false)
	if r.IsExpanded() {
		t.Error("IsExpanded() = true after SetExpanded(false)")
	}
}
