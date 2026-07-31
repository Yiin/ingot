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
// This package wires the seven flows the child spec names (capture,
// compose, copy, toggle, store-to-list-model, lock gating, lifecycle)
// plus the CLI's run/path/export/import commands and the no-permission
// degradation path. It deliberately does not wire internal/ui/menus'
// context/overflow menus, internal/ui/keymap's Nav-driven list
// navigation and project switcher, or internal/ui/searchbar's query
// into any actual filtering — those are real, separate scope (the last
// one is explicitly copper-l2z.28's own child), tracked as follow-ups
// (see the epic notes for this child). Space (mark done) and BackSpace
// (delete) on the list are wired directly via keymap.InstallListGate,
// since that primitive exists specifically to avoid stealing those keys
// from a focused text field. A row's own checkbox click IS wired (via
// List.ConnectToggled) — that's part of the store-to-list-model flow,
// not menus/keymap/search scope.
package app
