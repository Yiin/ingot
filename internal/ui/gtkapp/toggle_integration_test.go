//go:build integration

package gtkapp

import (
	"context"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"
)

// toggleRemoteHelperEnv marks a subprocess re-exec of this same test binary
// as the "second ingot invocation" in TestToggleRemoteActivatesPrimaryOnce,
// following the standard os/exec test-helper-process pattern.
const toggleRemoteHelperEnv = "GTKAPP_TOGGLE_REMOTE_HELPER"

// TestToggleRemoteActivatesPrimaryOnce starts an App as the primary
// instance, then runs a second process that calls ToggleRemote, and asserts
// the primary's action fired exactly once.
func TestToggleRemoteActivatesPrimaryOnce(t *testing.T) {
	appID := "lt.yiin.ingot.test.toggleremote"

	var activations atomic.Int32
	app := New(appID)

	app.OnStartup(func(a *App) {
		// Keep the main loop alive past the startup/activate signals;
		// without a hold, Run can exit before the remote's action ever
		// arrives. Quit (below) is what actually ends the loop.
		a.Application.Hold()
		a.AddAction("toggle", func() {
			activations.Add(1)
		})

		go func() {
			cmd := exec.Command(os.Args[0], "-test.run=TestToggleRemoteHelperProcess", "-test.v")
			cmd.Env = append(os.Environ(),
				toggleRemoteHelperEnv+"=1",
				"GTKAPP_TEST_APPID="+appID,
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("helper process failed: %v\n%s", err, out)
			}

			// Give the main loop a moment to actually dispatch the
			// incoming D-Bus ActivateAction call before quitting.
			time.Sleep(300 * time.Millisecond)
			a.Post(func() {
				a.Application.Quit()
			})
		}()
	})

	app.Run()

	if got := activations.Load(); got != 1 {
		t.Fatalf("primary action fired %d times, want exactly 1", got)
	}
}

// TestToggleRemoteHelperProcess is not a real test: it's re-exec'd as a
// subprocess by TestToggleRemoteActivatesPrimaryOnce to play the role of a
// second ingot invocation. Run directly, it just skips.
func TestToggleRemoteHelperProcess(t *testing.T) {
	if os.Getenv(toggleRemoteHelperEnv) != "1" {
		t.Skip("helper process for TestToggleRemoteActivatesPrimaryOnce, not a standalone test")
	}
	appID := os.Getenv("GTKAPP_TEST_APPID")

	sentRemote, err := ToggleRemote(context.Background(), appID, "toggle")
	if err != nil {
		t.Fatalf("ToggleRemote: %v", err)
	}
	if !sentRemote {
		t.Fatal("ToggleRemote reported sentRemote=false, want true: a primary instance should already be running")
	}
}
