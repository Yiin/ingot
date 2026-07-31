package gtkapp

import (
	"os"
	"sync/atomic"
	"syscall"
	"testing"
)

// requireDisplay skips when there is no Wayland or X11 display to
// connect to. Both tests below call App.Run, which runs gtk_init, and
// gtk_init without a display fails the whole package ("Failed to open
// display"). That made these look like ordinary unit tests locally,
// where a session is always present, while failing every headless CI
// `make check`. The integration job still runs them for real, inside
// scripts/headless.sh's sway session.
func requireDisplay(t *testing.T) {
	t.Helper()
	if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("DISPLAY") == "" {
		t.Skip("no WAYLAND_DISPLAY or DISPLAY; gtk_init needs one (run under scripts/headless.sh)")
	}
}

// newTestApp builds an App with a unique appID per test so parallel test
// binaries never contend for the same D-Bus well-known name.
func newTestApp(t *testing.T) *App {
	t.Helper()
	return New("lt.yiin.ingot.test." + t.Name())
}

// TestPostRunsOnRunGoroutineThread asserts Post's callback executes on the
// same OS thread as Run, per the package's thread-affinity contract.
func TestPostRunsOnRunGoroutineThread(t *testing.T) {
	requireDisplay(t)
	app := newTestApp(t)

	runTID := syscall.Gettid()
	var postTID int
	var ran atomic.Bool

	app.OnStartup(func(a *App) {
		// Without a hold, GApplication's use count is 0 after the startup
		// signal returns and Run may exit before the idle source below
		// ever gets a main-loop iteration to fire in.
		a.Hold()
		a.Post(func() {
			postTID = syscall.Gettid()
			ran.Store(true)
			a.Quit()
		})
	})

	app.Run()

	if !ran.Load() {
		t.Fatal("Post callback never ran")
	}
	if postTID != runTID {
		t.Fatalf("Post callback ran on TID %d, want the Run goroutine's TID %d", postTID, runTID)
	}
}

// TestActionPanicLeavesAppAlive asserts a panicking action handler is
// recovered rather than crashing the process, and that the app keeps
// running normally afterward.
func TestActionPanicLeavesAppAlive(t *testing.T) {
	requireDisplay(t)
	app := newTestApp(t)

	var afterPanicRan atomic.Bool

	app.OnStartup(func(a *App) {
		a.Hold()
		a.AddAction("boom", func() {
			panic("boom")
		})
		a.Post(func() {
			a.ActivateAction("boom", nil)
			a.Post(func() {
				afterPanicRan.Store(true)
				a.Quit()
			})
		})
	})

	app.Run()

	if !afterPanicRan.Load() {
		t.Fatal("app did not survive a panicking action handler")
	}
}
