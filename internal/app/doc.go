// Package app wires every other internal package into one running
// process: the evdev chord, the store, and the panel UI. This is
// Ingot's "one process" — see the epic architecture notes — the evdev
// reader and every other background goroutine run inside the same
// process as the GTK main loop, hopping onto it via internal/ui/gtkapp's
// App.Post (glib.IdleAdd) rather than through any IPC of their own.
//
// # Threading
//
// Everything gtkapp.App's own doc comment says applies here unchanged:
// exactly one goroutine (main's) ever touches GTK or GObject state, via
// App.Run. Every other goroutine — the evdev reader, the PRIMARY-read
// worker, the clipboard-write worker, the lock-state gate, the signal
// watcher — reaches back onto it only through App.Post. fsstore's
// Options.Post is wired to the same App.Post, so every store event
// (including a debounced save's own event) already arrives on the GTK
// thread by construction; the storeAdapter never hops itself.
//
// # cgo
//
// The epic restricts import "C" to internal/ui and internal/layershell,
// so that go test ./internal/store/... never links GTK. That rule
// constrains where cgo code lives, not who may import cgo-using
// packages: this package imports internal/ui/panel, internal/ui/gtkapp,
// and internal/layershell freely, and contains no import "C" of its
// own. The cost is that go test ./internal/app/... links GTK — nothing
// in this package's own tests requires a live display, the same as
// internal/ui/notelist's model-only tests, but the test binary is
// heavier to build.
//
// # Scope note
//
// This package wires the seven flows copper-l2z.30's own spec named
// (capture, compose, copy, toggle, store-to-list-model, lock gating,
// lifecycle) plus the CLI's run/path/export/import commands and the
// no-permission degradation path. copper-l2z.61 added the rest:
// internal/ui/menus' context/overflow menus and app-level actions
// (menus.go), internal/ui/keymap's Nav-driven list navigation — focus
// movement, jump-to-section, extend-selection, select-all-in-section —
// and the project switcher (nav.go, keys.go), and config.toml's [keys]
// per-action overrides plus panel.json's Keep on Top preference. Space
// (mark done) and BackSpace (delete) on the list are still wired
// directly via keymap.InstallListGate rather than through Nav or a menu
// action, since that primitive exists specifically to avoid stealing
// those keys from a focused text field.
//
// Still unwired, left for a future child: internal/ui/searchbar's query
// into any actual filtering (that's copper-l2z.28's own child, and
// unrelated to this package); keymap.HandleEscape/EscapeTarget's full
// six-step Escape cascade (searchbar's own OnEscapeAtEmpty covers only
// its own two steps today); a real note editor behind Edit/Edit in New
// Window/Expand (all three are Handlers stubs — see menus.go); and
// per-note reordering (move-note-up/move-note-down have Table entries
// and no store primitive to back them).
package app
