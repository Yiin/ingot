//go:build integration

// TestScreenshot_MapsAndCapturesANonUniformImage is copper-l2z.31's core
// acceptance criterion: open a layer-shell window, wait for it to map,
// assert IsSupported(), capture it with grim, and assert the PNG is not
// uniformly one colour. Deliberately no pixel-diffing — GTK's rendered
// output depends on fontconfig, the installed font set, hinting and the
// active renderer, so a golden-image gate would flake and then get
// ignored. "did it map and render something" is the regression this
// catches.
//
// Run this under scripts/headless.sh; `make screenshot` reuses this same
// test to (re)generate assets/screenshot.png once INGOT_SCREENSHOT_OUT is
// set, instead of duplicating the map-and-capture machinery.
package layershell

import (
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// pumpUntil drains the default main context, same idiom as
// internal/ui/notelist's pump(), but bounded by a deadline and re-checked
// against cond so callers can wait for an async compositor round trip
// (e.g. the surface actually mapping) instead of just draining whatever
// is already queued.
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
// compositor room to actually composite a frame after Mapped() flips —
// Mapped becoming true means the surface entered the map state, not that
// a frame has been painted yet.
func pumpFor(d time.Duration) {
	ctx := glib.MainContextDefault()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		for ctx.Iteration(false) {
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestScreenshot_MapsAndCapturesANonUniformImage(t *testing.T) {
	requireDisplay(t)
	requireLayerShell(t)

	if !IsSupported() {
		t.Fatal("IsSupported() is false, but requireLayerShell just confirmed the compositor supports wlr-layer-shell")
	}

	if _, err := exec.LookPath("grim"); err != nil {
		t.Skip("grim not installed")
	}

	win := gtk.NewWindow()
	win.SetChild(gtk.NewLabel("Ingot"))

	p, err := New(win, DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
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
