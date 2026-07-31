package menus

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
)

// TestBuildOverflowStructure checks the overflow menu's four groups: the
// project list, Clear Done, Keyboard Shortcuts, and a "Window" group
// rendered as a genuine section heading (not an item — see doc.go) with
// Keep on Top and Close inside it.
func TestBuildOverflowStructure(t *testing.T) {
	projects := []Project{{ID: "p1", Title: "Personal"}, {ID: "p2", Title: "Work"}}
	menu := BuildOverflow(projects)

	if n := menu.NItems(); n != 4 {
		t.Fatalf("overflow menu has %d top-level items, want 4 sections", n)
	}

	projSection := gio.BaseMenuModel(menu.ItemLink(0, "section"))
	if n := projSection.NItems(); n != len(projects) {
		t.Fatalf("project section has %d items, want %d", n, len(projects))
	}
	if got := itemLabel(t, projSection, 1); got != "Work" {
		t.Errorf("second project label = %q, want %q", got, "Work")
	}

	clearDone := gio.BaseMenuModel(menu.ItemLink(1, "section"))
	if got := itemLabel(t, clearDone, 0); got != "Clear Done" {
		t.Errorf("clear-done label = %q, want %q", got, "Clear Done")
	}

	shortcuts := gio.BaseMenuModel(menu.ItemLink(2, "section"))
	if got := itemLabel(t, shortcuts, 0); got != "Keyboard Shortcuts" {
		t.Errorf("shortcuts label = %q, want %q", got, "Keyboard Shortcuts")
	}

	// The "Window" section's own label is the parent item's "label"
	// attribute, not an item inside it — that is what makes it a
	// non-interactive heading instead of a disabled-looking row.
	windowLabel := itemLabel(t, menu, 3)
	if windowLabel != "Window" {
		t.Errorf("window section label = %q, want %q", windowLabel, "Window")
	}

	window := gio.BaseMenuModel(menu.ItemLink(3, "section"))
	if n := window.NItems(); n != 2 {
		t.Fatalf("window section has %d items, want 2 (Keep on Top, Close)", n)
	}
	if got := itemLabel(t, window, 0); got != "Keep on Top" {
		t.Errorf("window item 0 label = %q, want %q", got, "Keep on Top")
	}
	if got := itemLabel(t, window, 1); got != "Close" {
		t.Errorf("window item 1 label = %q, want %q", got, "Close")
	}
}
