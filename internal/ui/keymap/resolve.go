package keymap

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// accelKey is the normalised (keyval, default-masked modifiers) pair
// gtk.AcceleratorParse resolves an Entry's Accels string to — the key a
// real key controller or gtk.Application accelerator actually matches
// against, once GDK's incidental modifier bits (NumLock, ScrollLock,
// ...) are masked off by gtk.AcceleratorGetDefaultModMask.
type accelKey struct {
	val  uint
	mods gdk.ModifierType
}

// index builds scope's accelerator -> Entry lookup, or returns an error
// naming the first collision: two different actions in the same scope
// claiming the same physical key chord, which would make Resolve
// ambiguous. Table itself has none — see TestNoAccelCollisionsWithinScope
// — so this only ever errors on a future edit to Table.
func index(scope Scope) (map[accelKey]Entry, error) {
	out := make(map[accelKey]Entry)
	for _, e := range Table {
		if e.Scope != scope {
			continue
		}
		for _, accel := range e.Accels {
			val, mods, ok := gtk.AcceleratorParse(accel)
			if !ok {
				return nil, fmt.Errorf("keymap: %q (action %q) is not a valid accelerator", accel, e.Action)
			}
			key := accelKey{val, mods & gtk.AcceleratorGetDefaultModMask()}
			if existing, dup := out[key]; dup && existing.Action != e.Action {
				return nil, fmt.Errorf("keymap: %q collides between %q and %q", accel, existing.Action, e.Action)
			}
			out[key] = e
		}
	}
	return out, nil
}

// Resolve maps a real key-pressed event (keyval plus GDK's reported
// modifier state) back to the Table entry it triggers within scope, or
// reports false if no entry in that scope matches. This is the lookup a
// ScopeList key controller runs directly; a ScopeApp entry is instead
// installed as a real gtk.Application accelerator (see the package
// doc), with Resolve available for anything that wants to double-check
// what an accelerator string means. state is masked with
// gtk.AcceleratorGetDefaultModMask internally, so callers pass it
// through unmodified from ConnectKeyPressed.
func Resolve(scope Scope, keyval uint, state gdk.ModifierType) (Entry, bool) {
	idx, err := index(scope)
	if err != nil {
		// Table is a compile-time constant; a collision here is a
		// programming error caught by TestNoAccelCollisionsWithinScope,
		// never a runtime condition to recover from.
		panic(err)
	}
	e, ok := idx[accelKey{keyval, state & gtk.AcceleratorGetDefaultModMask()}]
	return e, ok
}
