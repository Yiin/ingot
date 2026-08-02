package app

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/keymap"
)

// TestGateHandlersAreReachableByTheirAccelerators walks the exact lookup
// keymap.InstallListGate performs at runtime: parse the accelerator the
// shortcuts window shows, feed the resulting (keyval, modifiers) to
// Resolve the way a GTK key controller would, and check it lands on an
// action the gate has a handler for.
//
// Wiring a handler under the right name is not enough on its own. The
// gate resolves against Table, so an accelerator string that parses to a
// chord Table indexes under some *other* action — or does not parse at
// all — leaves the handler unreachable while every other test still
// passes. This is the step between "the handler exists" and "GTK
// delivered the key", and it is the half that is our own logic.
//
// No display: AcceleratorParse and Resolve are both pure.
func TestGateHandlersAreReachableByTheirAccelerators(t *testing.T) {
	handlers := (&App{}).listActionHandlers()

	for action := range handlers {
		e, ok := keymap.ByAction(action)
		if !ok {
			t.Errorf("gate handles %q, which is not a keymap.Table action", action)
			continue
		}
		if e.Scope != keymap.ScopeList {
			t.Errorf("gate handles %q, but its Table entry is not ScopeList — InstallListGate resolves ScopeList only", action)
			continue
		}
		if len(e.Accels) == 0 {
			t.Errorf("gate handles %q, but its Table entry has no accelerator, so no key can ever reach it", action)
			continue
		}

		for _, accel := range e.Accels {
			val, mods, parsed := gtk.AcceleratorParse(accel)
			if !parsed {
				t.Errorf("%s: accelerator %q does not parse", action, accel)
				continue
			}
			got, found := keymap.Resolve(keymap.ScopeList, val, mods)
			if !found {
				t.Errorf("%s: %q parses but Resolve finds no ScopeList entry for it", action, accel)
				continue
			}
			if got.Action != action {
				t.Errorf("%s: %q resolves to %q instead, so the gate would run the wrong handler",
					action, accel, got.Action)
				continue
			}
			if handlers[got.Action] == nil {
				t.Errorf("%s: %q resolves to %q, which has no gate handler", action, accel, got.Action)
			}
		}
	}
}

// TestGateIgnoresKeysItHasNoHandlerFor pins the fall-through half of the
// contract. Down is ScopeList, but keymap.InstallNav implements it, so
// the gate must report it unhandled rather than swallowing it — otherwise
// arrow-key navigation would die the moment the gate was installed.
func TestGateIgnoresKeysItHasNoHandlerFor(t *testing.T) {
	handlers := (&App{}).listActionHandlers()

	for _, action := range []string{"focus-next", "focus-previous", "select-all-section"} {
		e, ok := keymap.ByAction(action)
		if !ok {
			t.Fatalf("keymap.Table has no %q entry", action)
		}
		if handlers[action] != nil {
			t.Errorf("gate claims %q, which keymap.InstallNav already implements — the two would both fire", action)
		}
		for _, accel := range e.Accels {
			val, mods, parsed := gtk.AcceleratorParse(accel)
			if !parsed {
				t.Fatalf("%s: accelerator %q does not parse", action, accel)
			}
			got, found := keymap.Resolve(keymap.ScopeList, val, mods)
			if found && handlers[got.Action] != nil {
				t.Errorf("%s: %q resolves to %q, which the gate would consume before Nav sees it",
					action, accel, got.Action)
			}
		}
	}
}
