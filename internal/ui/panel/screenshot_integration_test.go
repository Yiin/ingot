//go:build integration

// TestScreenshot_CapturesTheAssembledPanel is copper-l2z.38's acceptance
// criterion: assets/screenshot.png must be a genuine capture of the real
// running app, not a hand-built mockup. This maps the actual
// internal/ui/panel.Shell — the same widget tree internal/app wires up
// via internal/layershell.Panel — inside a headless compositor and
// captures it with grim, following copper-l2z.31's
// TestScreenshot_MapsAndCapturesANonUniformImage precedent
// (internal/layershell/screenshot_integration_test.go) but with real
// fixture content instead of a bare label, and reusing
// panel_integration_test.go's newFixtureSections() so the two tests
// can't drift about what "the panel's fixture content" means.
//
// Run via `make screenshot`, which sets INGOT_SCREENSHOT_OUT and drives
// scripts/headless.sh; needs a real GDK display and wlr-layer-shell
// support, so it is skipped rather than failed outside that harness.
package panel

import (
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/layershell"
	"github.com/Yiin/ingot/internal/ui/notelist"
	"github.com/Yiin/ingot/internal/ui/theme"
	"github.com/Yiin/ingot/internal/ui/toast"
)

// pumpUntil drains the default main context, same idiom as
// internal/layershell's own screenshot test, bounded by a deadline and
// re-checked against cond so the test can wait for the compositor's
// async map round trip instead of racing it.
func pumpUntil(timeout time.Duration, cond func() bool) bool {
	ctx := glib.MainContextDefault()
	deadline := time.Now().Add(timeout)
	for {
		for ctx.Iteration(false) {
		}
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return cond()
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// pumpFor drains the default main context for at least d, giving the
// compositor room to actually composite a frame after Mapped() flips.
func pumpFor(d time.Duration) {
	ctx := glib.MainContextDefault()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		for ctx.Iteration(false) {
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestScreenshot_CapturesTheAssembledPanel(t *testing.T) {
	display := requireDisplay(t)
	if !layershell.IsSupported() {
		t.Skip("compositor does not support wlr-layer-shell")
	}
	if _, err := exec.LookPath("grim"); err != nil {
		t.Skip("grim not installed")
	}
	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	panelToast := toast.NewInPanel()
	s := New(newFixtureSections(), "Notes", panelToast, toast.Nop{})

	// No row is left selected: .note-card.selected text currently
	// renders illegible (near-white on the light selected background —
	// see copper-l2z's discovered-work note on this child) which is a
	// theme/notelist CSS bug out of scope here, not something this
	// screenshot should showcase.
	a := notelist.NewItem("1", "inbox", "Draft the release notes for v0.1", false)
	b := notelist.NewItem("2", "inbox", "Fix the section sorter bug", true)
	c := notelist.NewItem("3", "work", "**Prompt:** rewrite the onboarding copy to be shorter and punchier. Focus on outcomes, not features.", false)
	d := notelist.NewItem("4", "ideas", "Wire the chord to capture, store, and panel end to end", false)
	s.List().Model().AppendAll([]*notelist.Item{a, b, c, d})
	// AppendAll doesn't drive the hint/list-visibility switch itself —
	// see RefreshEmptyState's doc comment, matchCount's source of truth
	// is always the caller — so without this the shell keeps showing
	// the first-run "Press Shift twice to capture" hint over a list that
	// now has real rows behind it.
	s.RefreshEmptyState("", 0)
	panelToast.Show("Copied 3 items to clipboard")

	win := gtk.NewWindow()
	win.SetChild(s.Widget())

	p, err := layershell.New(win, layershell.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("layershell.New: %v", err)
	}
	p.Show()

	if !pumpUntil(5*time.Second, win.Mapped) {
		t.Fatal("window did not map within 5s")
	}
	pumpFor(200 * time.Millisecond)

	out := os.Getenv("INGOT_SCREENSHOT_OUT")
	if out == "" {
		out = filepath.Join(t.TempDir(), "screenshot.png")
	} else if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatalf("creating %s: %v", filepath.Dir(out), err)
	}

	if output, err := exec.Command("grim", out).CombinedOutput(); err != nil {
		t.Fatalf("grim: %v: %s", err, output)
	}

	assertNotUniform(t, out)
}

// assertNotUniform fails t if the PNG at path has exactly one colour
// across every pixel — the "nothing actually rendered" regression this
// test exists to catch. It does not compare against any golden image.
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
