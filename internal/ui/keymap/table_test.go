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

// TestApplyOverrides mutates the package-level Table, so it saves and
// restores a deep-enough copy (Accels is the only field it ever writes)
// around itself — every other test in this file (and package) reads
// Table assuming it's still the real default.
func TestApplyOverrides(t *testing.T) {
	saved := make([][]string, len(Table))
	for i, e := range Table {
		saved[i] = e.Accels
	}
	t.Cleanup(func() {
		for i, accels := range saved {
			Table[i].Accels = accels
		}
	})

	rejected := ApplyOverrides(map[string]string{
		"mark-done":      "<Control>space",
		"no-such-action": "<Control>x",  // must be silently ignored, not rejected: not Table's job to warn
		"quit":           "",            // empty value must be ignored, not clear Accels
		"global-capture": "<Control>g",  // ScopeGlobal: no GTK accelerator to override, must reject
		"toggle-panel":   "not-a-chord", // bad syntax, must reject regardless of scope
		"focus-next":     "Up",          // collides with focus-previous's own "Up" within ScopeList, must reject
	})

	got, ok := ByAction("mark-done")
	if !ok {
		t.Fatal("ByAction(\"mark-done\") not found after ApplyOverrides")
	}
	if len(got.Accels) != 1 || got.Accels[0] != "<Control>space" {
		t.Errorf("mark-done Accels = %v, want [\"<Control>space\"]", got.Accels)
	}
	if _, rejected := rejected["mark-done"]; rejected {
		t.Error("mark-done was rejected, want accepted (valid, non-colliding, ScopeList)")
	}

	quit, ok := ByAction("quit")
	if !ok {
		t.Fatal("ByAction(\"quit\") not found")
	}
	if len(quit.Accels) != 1 || quit.Accels[0] != "<Control>q" {
		t.Errorf("quit Accels = %v, want unchanged [\"<Control>q\"] (empty override should be a no-op)", quit.Accels)
	}

	for _, action := range []string{"global-capture", "toggle-panel", "focus-next"} {
		if _, ok := rejected[action]; !ok {
			t.Errorf("action %q: not in the rejected map, want rejected", action)
		}
	}

	// Rejection must leave Table's own entry exactly as it was — this is
	// the crash-prevention property: Resolve's index build (called by
	// InstallNav on every keypress) assumes Table never collides within
	// a scope, so a rejected override reaching Table anyway would panic
	// on the very next keypress.
	if e, _ := ByAction("global-capture"); len(e.Accels) != 0 {
		t.Errorf("global-capture Accels = %v, want unchanged (empty)", e.Accels)
	}
	if e, _ := ByAction("toggle-panel"); len(e.Accels) != 1 || e.Accels[0] != "<Super><Shift>c" {
		t.Errorf("toggle-panel Accels = %v, want unchanged", e.Accels)
	}
	if e, _ := ByAction("focus-next"); len(e.Accels) != 1 || e.Accels[0] != "Down" {
		t.Errorf("focus-next Accels = %v, want unchanged [\"Down\"]", e.Accels)
	}

	// index(ScopeList) must still build cleanly after all of the above —
	// the actual property Resolve depends on every keypress.
	if _, err := index(ScopeList); err != nil {
		t.Errorf("index(ScopeList) after ApplyOverrides: %v, want no error", err)
	}

	// Every entry other than mark-done is untouched.
	for i, e := range Table {
		if e.Action == "mark-done" {
			continue
		}
		want := saved[i]
		if len(e.Accels) != len(want) {
			t.Errorf("action %q: Accels = %v, want unchanged %v", e.Action, e.Accels, want)
			continue
		}
		for j := range want {
			if e.Accels[j] != want[j] {
				t.Errorf("action %q: Accels = %v, want unchanged %v", e.Action, e.Accels, want)
				break
			}
		}
	}
}

// TestApplyOverridesDeterministicOnMutualCollision checks that when two
// overrides collide with each other, the outcome is deterministic
// (alphabetically-first action wins) rather than depending on Go's
// randomized map iteration order.
func TestApplyOverridesDeterministicOnMutualCollision(t *testing.T) {
	saved := make([][]string, len(Table))
	for i, e := range Table {
		saved[i] = e.Accels
	}
	t.Cleanup(func() {
		for i, accels := range saved {
			Table[i].Accels = accels
		}
	})

	for i := 0; i < 5; i++ {
		for i, accels := range saved {
			Table[i].Accels = accels
		}

		// Both ask for the same physical chord within ScopeList;
		// "first-note" sorts before "last-note".
		rejected := ApplyOverrides(map[string]string{
			"last-note":  "<Control><Shift>x",
			"first-note": "<Control><Shift>x",
		})

		if _, ok := rejected["first-note"]; ok {
			t.Fatalf("iteration %d: first-note (alphabetically first) was rejected, want accepted", i)
		}
		if _, ok := rejected["last-note"]; !ok {
			t.Fatalf("iteration %d: last-note was accepted, want rejected as a collision with first-note", i)
		}
	}
}
