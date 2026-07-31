package gtkapp

import (
	"sync/atomic"
	"syscall"
	"testing"
)

// newTestApp builds an App with a unique appID per test so parallel test
// binaries never contend for the same D-Bus well-known name.
func newTestApp(t *testing.T) *App {
	t.Helper()
	return New("lt.yiin.ingot.test." + t.Name())
}

// TestPostRunsOnRunGoroutineThread asserts Post's callback executes on the
// same OS thread as Run, per the package's thread-affinity contract.
func TestPostRunsOnRunGoroutineThread(t *testing.T) {
	app := newTestApp(t)

	runTID := syscall.Gettid()
	var postTID int
	var ran atomic.Bool

	app.OnStartup(func(a *App) {
		// Without a hold, GApplication's use count is 0 after the startup
		// signal returns and Run may exit before the idle source below
		// ever gets a main-loop iteration to fire in.
		a.Application.Hold()
		a.Post(func() {
			postTID = syscall.Gettid()
			ran.Store(true)
			a.Application.Quit()
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
	app := newTestApp(t)

	var afterPanicRan atomic.Bool

	app.OnStartup(func(a *App) {
		a.Application.Hold()
		a.AddAction("boom", func() {
			panic("boom")
		})
		a.Post(func() {
			a.ActivateAction("boom", nil)
			a.Post(func() {
				afterPanicRan.Store(true)
				a.Application.Quit()
			})
		})
	})

	app.Run()

	if !afterPanicRan.Load() {
		t.Fatal("app did not survive a panicking action handler")
	}
}
