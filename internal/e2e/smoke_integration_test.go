//go:build integration

// TestRun_StartsMapsCapturesAndSingleInstances is copper-l2z.76's
// regression test for commit 65265c3: three independent defects in the
// internal/app <-> internal/ui/gtkapp seam meant `ingot run` could not
// start at all, even though every package's own unit tests passed. Each
// defect is only visible from outside the process, so this launches the
// real built binary (not a library entry point) under
// scripts/headless.sh and drives it like a user would:
//
//   - defect 3 (no activate handler/hold, so Run returned immediately)
//     is caught by asserting the process is still alive several seconds
//     after launch;
//   - defects 1 and 2 (a throwaway probe GApplication blocking the real
//     one's registration, and registering before OnStartup was
//     connected) both mean no panel is ever built, caught by a grim
//     capture that must not be a single flat colour. sway 1.12's
//     `get_tree` does not surface layer-shell nodes at all (verified:
//     grepping a captured tree for "layer" or the panel's own
//     "ingot-panel" namespace never matches anything, panel or no
//     panel), so a non-uniform screenshot is the only way to observe
//     from outside the process that layershell.New actually mapped a
//     surface — the same method internal/layershell's own screenshot
//     test uses;
//   - the composer and store wiring is exercised end to end by typing
//     through wtype and reading the note back from the project
//     Markdown file, rather than trusting any in-process mock;
//   - a second invocation of the binary must exit 0 quickly via the
//     single-instance remote-activate path instead of trying (and
//     failing, or worse, succeeding) to become a second primary
//     instance.
//
// This package deliberately does not import gotk4 or any internal/ui
// package: copper-l2z's "only internal/ui and internal/layershell may
// use cgo" rule means this smoke test drives ingot as an external
// process, the same way a user's shell would.
package e2e

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// requireHeadlessSession skips the test outside scripts/headless.sh (or
// any other live Wayland session): WAYLAND_DISPLAY is what sway's own
// `exec` line sets for every process it launches, including go test.
func requireHeadlessSession(t *testing.T) {
	t.Helper()
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no WAYLAND_DISPLAY; run under scripts/headless.sh (make test-integration)")
	}
}

func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not on PATH", name)
	}
}

// buildIngot compiles the real cmd/ingot binary into a temp dir. Tests
// must launch this, not call app.Run in-process, or they would stop
// exercising the exact main -> cli.Run -> app.Run path commit 65265c3
// fixed.
func buildIngot(t *testing.T) string {
	t.Helper()

	modOut, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("go env GOMOD: %v", err)
	}
	root := filepath.Dir(strings.TrimSpace(string(modOut)))

	bin := filepath.Join(t.TempDir(), "ingot")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/ingot")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/ingot: %v\n%s", err, out)
	}
	return bin
}

// isolatedXDGEnv returns an environment for the binary under test that
// points every XDG dir at a fresh temp tree, so the test never reads or
// writes real notes, while keeping the inherited WAYLAND_DISPLAY,
// XDG_RUNTIME_DIR and PATH the headless session set up.
func isolatedXDGEnv(t *testing.T) (env []string, dataHome string) {
	t.Helper()
	base := t.TempDir()
	dataHome = filepath.Join(base, "data")
	configHome := filepath.Join(base, "config")
	stateHome := filepath.Join(base, "state")
	for _, d := range []string{dataHome, configHome, stateHome} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", d, err)
		}
	}
	env = append(os.Environ(),
		"XDG_DATA_HOME="+dataHome,
		"XDG_CONFIG_HOME="+configHome,
		"XDG_STATE_HOME="+stateHome,
	)
	return env, dataHome
}

// assertNotUniform fails t if the PNG at path is exactly one colour
// across every pixel — see internal/layershell's identically-named
// helper, which this mirrors: "did the panel actually render something"
// is the regression, not a pixel-perfect golden image.
func assertNotUniform(t *testing.T, path string) {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}

	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		t.Fatalf("%s decoded to a zero-sized image", path)
	}

	fr, fg, fb, fa := img.At(bounds.Min.X, bounds.Min.Y).RGBA()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if r != fr || g != fg || b != fb || a != fa {
				return
			}
		}
	}
	t.Fatalf("%s is uniformly one colour (rgba %d,%d,%d,%d) — nothing rendered", path, fr, fg, fb, fa)
}

func TestRun_StartsMapsCapturesAndSingleInstances(t *testing.T) {
	requireHeadlessSession(t)
	requireTool(t, "grim")
	requireTool(t, "wtype")

	bin := buildIngot(t)
	env, dataHome := isolatedXDGEnv(t)

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

	// Defect 3: no activate handler and no hold meant Run returned (and
	// the process exited) immediately.
	select {
	case err := <-exited:
		t.Fatalf("ingot run exited early instead of staying up (regression: commit 65265c3 defect 3, no activate handler/hold) — err=%v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	case <-time.After(4 * time.Second):
		// still running — good.
	}

	// Defects 1 and 2: a throwaway probe GApplication blocking the real
	// registration, and registering before OnStartup was connected,
	// both meant the panel was never built at all.
	shot := filepath.Join(t.TempDir(), "shot.png")
	if out, err := exec.Command("grim", shot).CombinedOutput(); err != nil {
		t.Fatalf("grim: %v: %s", err, out)
	}
	assertNotUniform(t, shot)

	// Drive the composer like a user would and assert the note lands on
	// disk with the on-disk grammar, exercising internal/app's wiring of
	// internal/ui/composer to the store end to end.
	const noteText = "ingot e2e smoke test note"
	if out, err := exec.Command("wtype", noteText).CombinedOutput(); err != nil {
		t.Fatalf("wtype: %v: %s", err, out)
	}
	if out, err := exec.Command("wtype", "-k", "Return").CombinedOutput(); err != nil {
		t.Fatalf("wtype -k Return: %v: %s", out, err)
	}

	projectsDir := filepath.Join(dataHome, "ingot", "projects")
	want := "- [ ] " + noteText
	if !pollForContent(t, projectsDir, want, 5*time.Second) {
		t.Fatalf("no *.md file under %s contained %q within 5s (composer -> store -> Markdown wiring broken)", projectsDir, want)
	}

	// Single-instance path: a second invocation must hand off to the
	// running instance via TryActivateRemote and exit 0 promptly,
	// never trying to become a second primary instance.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	second := exec.CommandContext(ctx, bin, "run")
	second.Env = env
	out, err := second.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("second `ingot run` invocation did not return within 5s; single-instance handoff appears to hang\n%s", out)
	}
	if err != nil {
		t.Fatalf("second `ingot run` invocation: %v\n%s", err, out)
	}

	// The first instance must still be the one running the panel.
	select {
	case err := <-exited:
		t.Fatalf("first instance exited after the second invocation (regression: single-instance handoff killed the primary) — err=%v", err)
	default:
	}
}

// pollForContent reports whether any *.md file in dir contains want
// within timeout — AddNote's write is debounce-bypassed for structural
// mutations, but this still polls rather than sleeping a fixed amount,
// since the exact write latency is not part of this test's contract.
func pollForContent(t *testing.T, dir, want string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		for _, m := range matches {
			data, err := os.ReadFile(m)
			if err == nil && strings.Contains(string(data), want) {
				return true
			}
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}
