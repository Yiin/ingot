package app

import (
	"log/slog"
	"runtime/debug"

	"github.com/Yiin/ingot/internal/store"
)

// guard returns a func to defer at the top of any callback GTK itself
// does not already wrap in a recover (gtkapp.App only covers OnStartup,
// AddAction, and Post) — every menu-less action handler and every
// widget signal internal/app connects directly. A panic escaping a
// GObject closure kills the process outright, and gotk4's own re-panic
// handler then SIGSEGVs, corrupting the stack trace, so it must never
// be allowed to propagate — see the epic architecture notes.
func guard(name string) func() {
	return func() {
		if r := recover(); r != nil {
			slog.Error("app: recovered panic", "in", name, "panic", r, "stack", string(debug.Stack()))
		}
	}
}

// safe wraps fn so a panic inside it is recovered and logged rather than
// propagating, for a callback registered once at wiring time (a widget
// signal, a store subscription).
func safe(name string, fn func()) func() {
	return func() {
		defer guard(name)()
		fn()
	}
}

// safeText is safe's counterpart for a callback that takes a string —
// composer.OnCommit's own signature. Go has no user-defined generics
// shortcut for "wrap any func(T)" that stays this readable at the call
// site, so each shape used gets its own tiny wrapper.
func safeText(name string, fn func(text string)) func(text string) {
	return func(text string) {
		defer guard(name)()
		fn(text)
	}
}

// safeEvent is safe's counterpart for store.Subscribe's callback shape.
func safeEvent(name string, fn func(store.Event)) func(store.Event) {
	return func(ev store.Event) {
		defer guard(name)()
		fn(ev)
	}
}

// goSafe starts fn on a new goroutine with the same panic recovery as
// safe. Every background goroutine this package starts — the evdev
// reader, the PRIMARY-read worker, the clipboard-write worker, the lock
// gate, the signal watcher — goes through this, since a panic here has
// no GTK callback wrapper to catch it at all.
func goSafe(name string, fn func()) {
	go safe(name, fn)()
}
