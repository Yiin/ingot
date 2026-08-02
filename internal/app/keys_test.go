package app

import (
	"testing"

	"github.com/Yiin/ingot/internal/ui/keymap"
	"github.com/Yiin/ingot/internal/ui/menus"
)

// handledByNav are the ScopeList actions keymap.InstallNav's own key
// controller implements, so they are deliberately absent from
// wireListGate's map. Kept as a list rather than derived because
// InstallNav switches on action names inside a closure, which is not
// readable as data — see internal/ui/keymap/navinstall.go.
var handledByNav = map[string]bool{
	"focus-next":            true,
	"focus-previous":        true,
	"jump-next-section":     true,
	"jump-previous-section": true,
	"first-note":            true,
	"last-note":             true,
	"extend-selection-down": true,
	"extend-selection-up":   true,
	"select-all-section":    true,
}

// knownUnwired are ScopeList accelerators the shortcuts window advertises
// that genuinely do nothing yet, each with the bead tracking it. This list
// is the point of the test: a gap here is a decision someone recorded, not
// a shortcut that quietly died. Shrink it, never grow it.
var knownUnwired = map[string]string{
	"move-note-up":   "copper-hfq", // no within-section reorder primitive on store.Store
	"move-note-down": "copper-hfq",
	"move-to":        "copper-d5w", // parameterised action, needs to open the submenu
}

// TestEveryListShortcutIsWired fails when keymap.Table advertises a
// list-scoped accelerator that nothing implements.
//
// The shortcuts window (Ctrl+?) is built straight from Table, so every
// entry there is a promise to the user. Nothing previously checked that
// the promise was kept, and four were not: Return, Ctrl+Return, Right and
// Left were listed as Edit inline, Edit in new window, Expand and Collapse
// while App.Edit, App.EditNewWindow and App.Expand were all empty stubs.
// Delete was listed for delete-note while the gate only ever matched
// BackSpace.
func TestEveryListShortcutIsWired(t *testing.T) {
	handlers := (&App{}).listActionHandlers()

	for _, e := range keymap.Table {
		if e.Scope != keymap.ScopeList || len(e.Accels) == 0 {
			continue
		}
		switch {
		case handlers[e.Action] != nil:
		case handledByNav[e.Action]:
		case hasLiveMenuAccel(e.Action):
		case knownUnwired[e.Action] != "":
			t.Logf("%s (%v) is a known gap, tracked by %s", e.Action, e.Accels, knownUnwired[e.Action])
		default:
			t.Errorf("keymap.Table advertises %q (%v) in the shortcuts window, but nothing implements it: "+
				"add it to wireListGate's map, or record it in knownUnwired with a bead", e.Action, e.Accels)
		}
	}
}

// TestKnownUnwiredAreStillUnwired is the other half: it fails once a gap
// gets implemented, so the list cannot rot into a lie about the app.
func TestKnownUnwiredAreStillUnwired(t *testing.T) {
	handlers := (&App{}).listActionHandlers()
	for action, bead := range knownUnwired {
		if handlers[action] != nil || hasLiveMenuAccel(action) {
			t.Errorf("%q is listed in knownUnwired (%s) but is wired now — delete the entry and close the bead", action, bead)
		}
		if _, ok := keymap.ByAction(action); !ok {
			t.Errorf("knownUnwired names %q, which is not a keymap.Table action at all", action)
		}
	}
}

// hasLiveMenuAccel reports whether menus.Register leaves tableAction with
// a working app-wide accelerator.
//
// Listing it in menus.Accels is not enough: wireMenus revokes some of
// them again (accelsRevokedForListGate) because the gate carries those
// keys correctly instead. Reading only menus.Accels is what made the
// first version of TestEveryListShortcutIsWired pass with
// App.EditNewWindow deleted from the gate — the accelerator it was
// trusting had been taken back at startup.
func hasLiveMenuAccel(tableAction string) bool {
	name := menuActionName(tableAction)
	if len(menus.Accels[name]) == 0 {
		return false
	}
	for _, revoked := range accelsRevokedForListGate {
		if revoked == name {
			return false
		}
	}
	return true
}

// menuActionName maps a keymap.Table action to the gio action name
// menus.Register installs for it, for the two that differ. Same mismatch
// keyOverrideActionAliases exists for, in the other direction: Table's
// "edit-inline" is menus' "edit", and Table's "hide-panel" is "close".
func menuActionName(tableAction string) string {
	switch tableAction {
	case "edit-inline":
		return "edit"
	case "hide-panel":
		return "close"
	}
	return tableAction
}
