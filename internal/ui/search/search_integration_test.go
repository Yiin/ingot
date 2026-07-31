//go:build integration

// Apply's own decision logic is unit-tested against compute directly in
// search_test.go, with no display needed. This file exercises the thin
// GTK-effecting wrapper around it — SetFilter/RefreshHighlights/
// SelectItems/SetAnchor/ScrollTo actually reaching a live notelist.List —
// which needs a real GDK display to even construct (see notelist's own
// list_integration_test.go). This worktree has no display at all: this
// file only needs to compile here (go vet -tags integration), never
// execute — same convention as every other *_integration_test.go in this
// repo.
package search

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/notelist"
)

func requireGTKInit(t *testing.T) {
	t.Helper()
	gtk.Init()
}

func TestApplyHidesSectionsWithNoMatchLive(t *testing.T) {
	requireGTKInit(t)

	list := notelist.New([]notelist.Section{
		{ID: "inbox", Title: "Inbox"},
		{ID: "work", Title: "Work"},
	})
	a := notelist.NewItem("a", "inbox", "buy milk", false)
	b := notelist.NewItem("b", "work", "unrelated report", false)
	list.Model().AppendAll([]*notelist.Item{a, b})

	c := New(list)
	n := c.Apply("milk")
	if n != 1 {
		t.Fatalf("Apply(%q) = %d, want 1", "milk", n)
	}
	if got := list.Selection().NItems(); got != 1 {
		t.Errorf("Selection().NItems() = %d, want 1", got)
	}

	n = c.Apply("")
	if n != 2 {
		t.Fatalf("Apply(\"\") = %d, want 2", n)
	}
	if got := list.Selection().NItems(); got != 2 {
		t.Errorf("Selection().NItems() = %d, want 2", got)
	}
}

func TestApplyMovesFocusToFirstMatchLive(t *testing.T) {
	requireGTKInit(t)

	list := notelist.New([]notelist.Section{{ID: "a", Title: "A"}})
	x := notelist.NewItem("x", "a", "keep this", false)
	y := notelist.NewItem("y", "a", "hidden by search", false)
	list.Model().AppendAll([]*notelist.Item{x, y})

	list.SelectItems([]*notelist.Item{y})
	list.SetAnchor(y)

	c := New(list)
	c.Apply("keep")

	if got := list.Anchor(); got != x {
		t.Errorf("Anchor() = %v, want %v", got, x)
	}
}
