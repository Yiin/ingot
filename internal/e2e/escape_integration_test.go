//go:build integration

// TestEscape_HidesThePanel is copper-l2z.85's regression test.
//
// keymap.HandleEscape and its EscapeTarget cascade were implemented and
// fully unit tested, and nothing in the running app ever called them —
// grep for a caller outside internal/ui/keymap returned a single doc
// comment. Escape did nothing, so once the user clicked into the
// composer the only way back out was Ctrl+W.
//
// Unit tests structurally cannot catch that: keymap's own tests pass
// against a fake EscapeTarget whether or not a real one is ever
// installed. Only driving the built binary from outside proves the key
// is wired, so this lives beside the startup smoke test rather than in
// internal/app.
//
// The cascade's last step is the one asserted here: with no popover
// open, no inline edit, no search text, no selection, and the composer
// already focused, Escape falls through to HidePanel. Hiding is
// observable from outside the process as a screen that goes back to one
// flat colour, the same signal the smoke test uses in reverse to prove
// the panel mapped.
package e2e

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestEscape_HidesThePanel(t *testing.T) {
	requireHeadlessSession(t)
	requireTool(t, "grim")
	requireTool(t, "wtype")

	bin := buildIngot(t)
	env, _ := isolatedXDGEnv(t)

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, "run")
	cmd.Env = env
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting %s run: %v", bin, err)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-exited:
			case <-time.After(3 * time.Second):
				_ = cmd.Process.Kill()
			}
		}
	})

	select {
	case err := <-exited:
		t.Fatalf("ingot run exited early — err=%v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	case <-time.After(4 * time.Second):
	}

	// Panel is up: the screen must not be one flat colour.
	before := filepath.Join(t.TempDir(), "before.png")
	if out, err := exec.Command("grim", before).CombinedOutput(); err != nil {
		t.Fatalf("grim (before): %v: %s", err, out)
	}
	assertNotUniform(t, before)

	// Escape from the composer, which the panel focuses on first run.
	// The throwaway leading key is copper-l2z.83's finding: a fresh wtype
	// process drops its own first key event, so a bare `wtype -k Escape`
	// would send nothing at all.
	if out, err := exec.Command("wtype", "-k", "End", "-s", "300", "-k", "Escape").CombinedOutput(); err != nil {
		t.Fatalf("wtype -k Escape: %v: %s", err, out)
	}

	if !pollForUniform(t, 5*time.Second) {
		t.Fatal("panel still on screen 5s after Escape (regression: keymap.HandleEscape is not wired into the running app — copper-l2z.85)")
	}

	// Hiding must not be quitting: the chord has to stay armed.
	select {
	case err := <-exited:
		t.Fatalf("ingot exited on Escape instead of hiding the panel — err=%v\nstderr:\n%s", err, stderr.String())
	default:
	}
}

// pollForUniform reports whether the screen becomes a single flat colour
// within timeout. It polls rather than sleeping a fixed amount because
// the hide animation's duration is not part of this test's contract.
func pollForUniform(t *testing.T, timeout time.Duration) bool {
	t.Helper()
	dir := t.TempDir()
	deadline := time.Now().Add(timeout)
	for i := 0; ; i++ {
		shot := filepath.Join(dir, "poll.png")
		if out, err := exec.Command("grim", shot).CombinedOutput(); err != nil {
			t.Fatalf("grim (poll): %v: %s", err, out)
		}
		if isUniform(t, shot) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(250 * time.Millisecond)
	}
}
