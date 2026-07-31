package keymap

import (
	"slices"
	"testing"
)

func TestNavActionDispatchesEveryMappedAction(t *testing.T) {
	cases := []struct {
		action    string
		wantFocus string
	}{
		{"focus-next", "n1"},
		{"focus-next", "n2"},
		{"jump-next-section", "n4"},
		{"jump-previous-section", "n1"},
		{"last-note", "n6"},
		{"first-note", "n1"},
		{"focus-previous", "n1"}, // no-op at the first row
		{"extend-selection-down", "n2"},
		{"extend-selection-up", "n1"},
		{"select-all-section", "n1"},
	}

	n := NewNav(fixtureRows)
	for _, c := range cases {
		if !navAction(n, c.action) {
			t.Fatalf("navAction(%q) reported unhandled, want handled", c.action)
		}
		if got := n.FocusedID(); got != c.wantFocus {
			t.Errorf("after navAction(%q): focus = %q, want %q", c.action, got, c.wantFocus)
		}
	}
}

func TestNavActionReportsUnhandledForNonNavActions(t *testing.T) {
	n := NewNav(fixtureRows)
	for _, action := range []string{"mark-done", "delete-note", "edit-inline", "expand", "copy", ""} {
		if navAction(n, action) {
			t.Errorf("navAction(%q) reported handled, want unhandled", action)
		}
	}
}

func TestNavSyncFocus(t *testing.T) {
	n := NewNav(fixtureRows)
	n.FocusNext() // focus n1

	n.SyncFocus("n5")
	if got := n.Focus(); got != 4 {
		t.Fatalf("Focus() after SyncFocus(n5) = %d, want 4", got)
	}
	if got := n.FocusedID(); got != "n5" {
		t.Fatalf("FocusedID() after SyncFocus(n5) = %q, want n5", got)
	}

	// A following keyboard move continues from the synced row, not from
	// wherever keyboard focus was before the external click.
	n.FocusNext()
	if got := n.FocusedID(); got != "n6" {
		t.Errorf("FocusedID() after FocusNext following SyncFocus = %q, want n6", got)
	}

	// The current selection is untouched by the sync itself.
	n.ClearSelection()
	n.SyncFocus("n2")
	if n.HasSelection() {
		t.Error("SyncFocus should not itself change the selection")
	}

	// An unknown id is a no-op.
	n.SyncFocus("does-not-exist")
	if got := n.FocusedID(); got != "n2" {
		t.Errorf("SyncFocus with an unknown id changed focus to %q, want unchanged n2", got)
	}
}

// TestNavSyncSelection guards the fix for the bug a mouse click exposes
// otherwise: without SyncSelection, Nav's own selection map only ever
// reflects Nav's own prior keyboard moves, so a mouse-driven selection
// change would leave Ctrl+A/extend operating against a stale base set.
func TestNavSyncSelection(t *testing.T) {
	n := NewNav(fixtureRows)
	n.FocusNext() // focus+select n1, so Nav starts with its own idea of the selection

	// A mouse click selected n3..n5 (e.g. a Shift+click range) — sync
	// that in.
	n.SyncSelection([]string{"n3", "n4", "n5"})

	if got := n.Selected(); !slices.Equal(got, []string{"n3", "n4", "n5"}) {
		t.Fatalf("Selected() after SyncSelection = %v, want [n3 n4 n5]", got)
	}
	if got := n.FocusedID(); got != "n5" {
		t.Fatalf("FocusedID() after SyncSelection = %q, want n5 (the last synced id)", got)
	}

	// A following keyboard extend ranges from the synced anchor (n5, the
	// last synced id — see SyncFocus), not from Nav's own stale n1
	// anchor: ExtendDown moves focus to n6 and selects the anchor..focus
	// range, replacing (not adding to) the synced n3..n5 selection —
	// the same anchor-is-a-single-point behavior RangeClick has.
	n.ExtendDown()
	if got := n.Selected(); !slices.Equal(got, []string{"n5", "n6"}) {
		t.Errorf("Selected() after ExtendDown following SyncSelection = %v, want [n5 n6]", got)
	}

	// An id not present in the current row order is dropped, not just
	// left unresolved.
	n.SyncSelection([]string{"n1", "does-not-exist", "n2"})
	if got := n.Selected(); !slices.Equal(got, []string{"n1", "n2"}) {
		t.Errorf("Selected() after SyncSelection with a stale id = %v, want [n1 n2]", got)
	}

	// Syncing an empty selection clears it without moving focus, same as
	// ClearSelection.
	n.SyncSelection(nil)
	if n.HasSelection() {
		t.Error("SyncSelection(nil) should clear the selection")
	}
	if got := n.FocusedID(); got != "n2" {
		t.Errorf("FocusedID() after SyncSelection(nil) = %q, want unchanged n2 (focus untouched)", got)
	}
}
