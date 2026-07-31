// Package gtkapp is the process skeleton every other internal/ui package
// plugs into: a single-instance GtkApplication plus a way for other
// goroutines to reach the GTK thread.
//
// # Thread affinity
//
// gtk/v4's init() calls runtime.LockOSThread() (gtk.go:146892), which locks
// whichever goroutine imports the package to its OS thread. Since init()
// runs on the initial goroutine, [App.Run] must be called from main's
// goroutine — never from inside a go func(){} — and every GTK or GObject
// call must happen on that same goroutine for as long as the process runs.
// There is no lock and no runtime check guarding this: a violating call is
// a data race, not an error that surfaces cleanly. [App.Post] (wrapping
// glib.IdleAdd) is the only sanctioned way for another goroutine to reach
// the GTK thread; glib.TimeoutAdd is the delayed equivalent.
//
// # Init order
//
// GTK must be initialised before any widget call. Calling e.g.
// gtk.NewLabel("x") before [App.Run] has started GTK SIGSEGVs inside
// gtk_label_new with no usable message, because GTK's global state doesn't
// exist yet. Build widgets from inside an [App.OnStartup] callback (or a
// GtkApplication "activate"/"open" handler), never at package scope or
// before Run.
package gtkapp

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// App wraps a *gtk.Application built for exactly one Ingot process. Every
// method that touches the underlying GtkApplication must be called from the
// goroutine that will call (or already called) Run — see the package
// comment.
type App struct {
	*gtk.Application
}

// New builds an App for appID. It does not touch GTK's global state or the
// D-Bus session bus yet; that happens inside Run.
func New(appID string) *App {
	return &App{Application: gtk.NewApplication(appID, gio.ApplicationFlagsNone)}
}

// OnStartup registers fn to run once, when this process becomes the primary
// instance and GTK has just finished initialising. This is where widgets
// must be built — see the package comment on init order. A panic inside fn
// is recovered so it cannot bring down the process.
func (a *App) OnStartup(fn func(*App)) {
	a.ConnectStartup(func() {
		a.safeCall(func() { fn(a) })
	})
}

// OnActivate registers fn to run on GApplication's "activate" signal: once
// during Run, and again every time a second ingot invocation calls
// [App.TryActivateRemote] with no action name, or the desktop file is
// launched again.
//
// Connecting activate is not optional. A GApplication with no activate
// handler makes g_application_run warn "Your application does not implement
// g_application_activate()" and return immediately, so the process exits
// before the panel is ever usable. A panic inside fn is recovered.
func (a *App) OnActivate(fn func(*App)) {
	a.ConnectActivate(func() {
		a.safeCall(func() { fn(a) })
	})
}

// KeepAlive holds the application open independently of its windows, and
// returns a function that drops the hold.
//
// Ingot needs this because its whole point is to sit idle with the panel
// hidden while the global Shift-Shift chord stays armed. GApplication exits
// once its last visible window goes away, so without a hold, hiding the
// panel would quit the process and disarm capture.
func (a *App) KeepAlive() (release func()) {
	a.Hold()
	var once sync.Once
	return func() { once.Do(a.Release) }
}

// AddAction registers a named, parameterless action (e.g. "toggle") that
// ToggleRemote can activate from a second process. Prefer a named action
// over relying on GtkApplication's built-in "activate" signal: activate
// also fires on ordinary startup, so it cannot distinguish "first launch"
// from "toggle me" without extra state. A panic inside fn is recovered so
// it cannot bring down the process.
func (a *App) AddAction(name string, fn func()) {
	action := gio.NewSimpleAction(name, nil)
	action.ConnectActivate(func(_ *glib.Variant) {
		a.safeCall(fn)
	})
	a.Application.AddAction(action)
}

// Run starts the application on the calling goroutine, which must be
// main's goroutine (see the package comment). It returns the process exit
// code. It calls the underlying Run with a nil argv, not os.Args: GApplication
// would otherwise try to parse them as its own command-line options.
func (a *App) Run() int {
	return a.Application.Run(nil)
}

// Post schedules fn to run on the GTK thread via glib.IdleAdd, so goroutines
// other than the one running Run can safely touch GTK or GObject state. A
// panic inside fn is recovered so it cannot bring down the process.
func (a *App) Post(fn func()) {
	glib.IdleAdd(func() {
		a.safeCall(fn)
	})
}

// safeCall runs fn and recovers any panic. A panic escaping a GObject
// closure kills the process outright, and gotk4's re-panic handler then
// SIGSEGVs, corrupting the stack trace — so every user callback must be
// wrapped here rather than left to propagate.
func (a *App) safeCall(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("gtkapp: recovered panic in callback", "panic", r)
		}
	}()
	fn()
}

// mainContextFlushBudget bounds how long ToggleRemote pumps the default
// main context to flush the ActivateAction D-Bus call before returning.
const mainContextFlushBudget = 100 * time.Millisecond

// TryActivateRemote registers THIS App on the session bus and, if another
// instance already owns the application id, activates actionName on it and
// reports sentRemote=true so the caller knows to exit without building a
// window. If sentRemote is false, this process is the primary instance and
// the caller should carry on wiring actions and call Run as normal.
//
// Register the real App, never a throwaway one. g_application_register
// exports an org.gtk.Application object at the object path derived from the
// application id, and a second GApplication carrying the same id cannot
// export at that same path — it fails with "An object is already exported
// for the interface org.gtk.Application at /...", which makes starting a
// primary instance impossible. Registering here is safe and idempotent:
// g_application_run calls g_application_register itself and treats an
// already-registered application as a no-op, and actions added after
// registration are still exported, because GApplication keeps its exported
// action group in sync.
//
// ctx must not be nil: gotk4 v0.4.0's core/gcancel panics converting a nil
// context. A nil ctx is replaced with context.Background().
func (a *App) TryActivateRemote(ctx context.Context, actionName string) (sentRemote bool, err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := a.Register(ctx); err != nil {
		return false, err
	}
	if !a.IsRemote() {
		return false, nil
	}

	a.ActivateAction(actionName, nil)

	// ActivateAction queues an async D-Bus call; without pumping the
	// context, the process can exit before the call is ever written to the
	// socket and the primary instance never sees it.
	mainCtx := glib.MainContextDefault()
	deadline := time.Now().Add(mainContextFlushBudget)
	for time.Now().Before(deadline) {
		if !mainCtx.Iteration(false) {
			break
		}
	}

	return true, nil
}
