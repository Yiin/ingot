package keymap

import "testing"

// TestSpaceGating is the acceptance criteria's "Space types a space
// when the composer is focused and toggles done when the list is
// focused" — checked at the policy level ShouldGateForList decides at:
// a real key controller calls IsTextFocused for textFocused, which
// go test cannot exercise with no live display (see the package doc),
// but the decision itself is pure and is what's under test here.
func TestSpaceGating(t *testing.T) {
	t.Run("composer (or any text field) focused: space types, not gated to the list", func(t *testing.T) {
		if ShouldGateForList(true) {
			t.Errorf("ShouldGateForList(textFocused=true) = true, want false — space must type into the focused field")
		}
	})
	t.Run("list focused, no text field: space is gated to mark-done", func(t *testing.T) {
		if !ShouldGateForList(false) {
			t.Errorf("ShouldGateForList(textFocused=false) = false, want true — space must toggle done")
		}
	})
}

// TestMarkDoneAndDeleteAreListScopedNotAppAccelerators guards the other
// half of the same gating story: mark-done (Space) and delete-note
// (Delete/BackSpace) must never be installed as gtk.Application-wide
// accelerators, or they would fire while any text field is focused —
// exactly what the key-controller gating in gating.go exists to avoid.
func TestMarkDoneAndDeleteAreListScopedNotAppAccelerators(t *testing.T) {
	for _, action := range []string{"mark-done", "delete-note"} {
		e, ok := ByAction(action)
		if !ok {
			t.Fatalf("Table has no %q entry", action)
		}
		if e.Scope != ScopeList {
			t.Errorf("action %q has Scope %v, want ScopeList", action, e.Scope)
		}
	}
}

// TestSelectAllInSectionIsListScoped guards the analogous Ctrl+A story:
// "must select text in a field" per the spec means Ctrl+A only selects
// every note in a section while the list itself is focused, never
// stealing a text field's own select-all.
func TestSelectAllInSectionIsListScoped(t *testing.T) {
	e, ok := ByAction("select-all-section")
	if !ok {
		t.Fatalf("Table has no %q entry", "select-all-section")
	}
	if e.Scope != ScopeList {
		t.Errorf("action %q has Scope %v, want ScopeList", e.Action, e.Scope)
	}
}
