package keymap

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestActionsAreUnique(t *testing.T) {
	seen := make(map[string]bool, len(Table))
	for _, e := range Table {
		if seen[e.Action] {
			t.Errorf("Table has a duplicate Action %q", e.Action)
		}
		seen[e.Action] = true
	}
}

func TestTitlesAreUnique(t *testing.T) {
	seen := make(map[string]bool, len(Table))
	for _, e := range Table {
		if seen[e.Title] {
			t.Errorf("Table has a duplicate Title %q", e.Title)
		}
		seen[e.Title] = true
	}
}

func TestEveryEntryHasEitherAccelsOrDisplay(t *testing.T) {
	for _, e := range Table {
		if len(e.Accels) == 0 && e.Display == "" {
			t.Errorf("Table entry %q has neither Accels nor Display — the shortcuts window would render it blank", e.Action)
		}
	}
}

func TestEveryEntryIsInADeclaredGroup(t *testing.T) {
	declared := make(map[Group]bool, len(Groups))
	for _, g := range Groups {
		declared[g] = true
	}
	for _, e := range Table {
		if !declared[e.Group] {
			t.Errorf("Table entry %q has Group %q, which is not in Groups — it would be silently dropped from the shortcuts window", e.Action, e.Group)
		}
	}
}

// TestNoAccelCollisionsWithinScope guards index/Resolve's core
// invariant: within one Scope, every accelerator names exactly one
// action.
func TestNoAccelCollisionsWithinScope(t *testing.T) {
	for _, scope := range []Scope{ScopeApp, ScopeList} {
		if _, err := index(scope); err != nil {
			t.Errorf("scope %v: %v", scope, err)
		}
	}
}

// TestAccelsAreUnique checks the stronger, whole-Table property: no
// accelerator string is reused by a second action anywhere in Table,
// even across scopes — Table is the single source of truth for every
// binding, so an accelerator reused for two different actions, even in
// different scopes, would be confusing and is never intentional here.
func TestAccelsAreUnique(t *testing.T) {
	owner := make(map[string]string)
	for _, e := range Table {
		for _, accel := range e.Accels {
			if prev, dup := owner[accel]; dup {
				t.Errorf("accelerator %q is bound to both %q and %q", accel, prev, e.Action)
			}
			owner[accel] = e.Action
		}
	}
}

// TestResolveRoundTrip checks that every ScopeApp/ScopeList entry's own
// accelerators resolve straight back to that same entry — the
// bidirectional guarantee the acceptance criteria calls for, minus the
// GtkShortcutsWindow half covered in shortcuts_test.go.
func TestResolveRoundTrip(t *testing.T) {
	for _, e := range Table {
		if e.Scope != ScopeApp && e.Scope != ScopeList {
			continue
		}
		for _, accel := range e.Accels {
			val, mods, ok := gtk.AcceleratorParse(accel)
			if !ok {
				t.Fatalf("action %q: %q does not parse as a GTK accelerator", e.Action, accel)
			}
			got, found := Resolve(e.Scope, val, mods)
			if !found {
				t.Errorf("Resolve(%v, %q) found nothing, want action %q", e.Scope, accel, e.Action)
				continue
			}
			if got.Action != e.Action {
				t.Errorf("Resolve(%v, %q) = action %q, want %q", e.Scope, accel, got.Action, e.Action)
			}
		}
	}
}

// TestNoAvoidedBindings checks that no ScopeApp or ScopeList entry
// resolves to the same physical key chord as an AvoidList entry — see
// AvoidList's doc for why each one collides with GTK or IBus.
func TestNoAvoidedBindings(t *testing.T) {
	for _, avoid := range AvoidList {
		val, mods, ok := gtk.AcceleratorParse(avoid)
		if !ok {
			t.Fatalf("AvoidList entry %q does not parse as a GTK accelerator", avoid)
		}
		for _, scope := range []Scope{ScopeApp, ScopeList} {
			if e, found := Resolve(scope, val, mods); found {
				t.Errorf("Table binds %q (action %q, scope %v) to a chord on AvoidList", avoid, e.Action, scope)
			}
		}
	}
}

func TestByAction(t *testing.T) {
	e, ok := ByAction("mark-done")
	if !ok || e.Action != "mark-done" {
		t.Fatalf("ByAction(\"mark-done\") = %+v, %v", e, ok)
	}
	if _, ok := ByAction("no-such-action"); ok {
		t.Errorf("ByAction(\"no-such-action\") found something, want not found")
	}
}
