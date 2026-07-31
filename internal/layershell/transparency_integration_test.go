//go:build integration

// TestPanelWindowCornersAreTransparent pins the fix for the panel
// rendering as a rectangle with dark corners.
//
// .ingot-panel rounds its own corners, but GTK paints every GtkWindow an
// opaque background from the theme, and under layer-shell the surface is
// exactly the panel's size. The window's square therefore showed through
// at all four corners, so the rounding was purely cosmetic — measured on
// the real compositor before the fix as a flat rgba(52,52,52) at every
// corner, identical on all four and unrelated to the wallpaper behind.
//
// theme's stylesheet clears that background for
// theme.PanelWindowClass. This test proves the clearing works by putting
// a known solid colour behind the panel and asserting the corners still
// show it while the centre does not.
package layershell

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// bgColour is the solid background painted behind the panel. It is
// deliberately a saturated colour no part of the light theme uses, so
// "the corner is still this colour" cannot pass by coincidence.
const bgColour = "#ff00ff"

var wantBG = [3]uint8{0xff, 0x00, 0xff}

func rgb(img image.Image, x, y int) [3]uint8 {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}

// near reports whether c is within tol of want on every channel. The
// compositor composites in its own colour space and the panel casts a
// drop shadow that darkens nearby background, so an exact match is the
// wrong assertion; "still recognisably the background, not window
// chrome" is the real contract.
func near(c, want [3]uint8, tol int) bool {
	for i := range c {
		d := int(c[i]) - int(want[i])
		if d < 0 {
			d = -d
		}
		if d > tol {
			return false
		}
	}
	return true
}

func TestPanelWindowCornersAreTransparent(t *testing.T) {
	requireDisplay(t)
	requireLayerShell(t)

	if _, err := exec.LookPath("grim"); err != nil {
		t.Skip("grim not installed")
	}
	if _, err := exec.LookPath("swaymsg"); err != nil {
		t.Skip("swaymsg not installed; this test needs sway to paint a known background")
	}

	if out, err := exec.Command("swaymsg", "output", "*", "bg", bgColour, "solid_color").CombinedOutput(); err != nil {
		t.Skipf("could not set a solid background (not running under sway?): %v: %s", err, out)
	}

	// The rules under test live in theme's stylesheet, so it has to be
	// installed here exactly as gtkapp.Run installs it in production.
	if err := theme.Load(gdk.DisplayGetDefault()); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	win := gtk.NewWindow()
	t.Cleanup(win.Destroy)
	win.AddCSSClass(theme.PanelWindowClass)

	// A child that fills the window and paints the panel's own opaque
	// background, exactly as panel.Shell's root does. Without the
	// stylesheet rule under test, the window paints behind this and the
	// corners come back as window chrome instead of the background.
	child := gtk.NewBox(gtk.OrientationVertical, 0)
	child.AddCSSClass("ingot-panel")
	child.SetHExpand(true)
	child.SetVExpand(true)
	win.SetChild(child)

	p, err := New(win, DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p.Show()

	if !pumpUntil(5*time.Second, win.Mapped) {
		t.Fatal("window did not map within 5s")
	}
	pumpFor(300 * time.Millisecond)

	shot := filepath.Join(t.TempDir(), "corners.png")
	if out, err := exec.Command("grim", shot).CombinedOutput(); err != nil {
		t.Fatalf("grim: %v: %s", err, out)
	}

	f, err := os.Open(shot)
	if err != nil {
		t.Fatalf("opening %s: %v", shot, err)
	}
	defer func() { _ = f.Close() }()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decoding %s: %v", shot, err)
	}

	// Sanity: the background really is what we asked for, well away from
	// the panel (which is anchored to the right edge).
	b := img.Bounds()
	if got := rgb(img, b.Min.X+2, b.Min.Y+2); !near(got, wantBG, 8) {
		t.Skipf("background is %v, not the requested %v — compositor ignored the bg request, nothing to assert against", got, wantBG)
	}

	// Locate the panel on the middle row. It is anchored to the right
	// edge but carries a margin, so it does not reach x = width-1: scan
	// left past the margin to the panel's right edge, then keep going to
	// its left edge.
	midY := b.Min.Y + b.Dy()/2
	right := -1
	for x := b.Max.X - 1; x >= b.Min.X; x-- {
		if !near(rgb(img, x, midY), wantBG, 24) {
			right = x
			break
		}
	}
	left := -1
	if right >= 0 {
		left = b.Min.X
		for x := right; x >= b.Min.X; x-- {
			if near(rgb(img, x, midY), wantBG, 24) {
				left = x + 1
				break
			}
		}
	}
	if right < 0 || left < 0 || right-left < 40 {
		var sample string
		for x := b.Max.X - 1; x > b.Max.X-400 && x >= b.Min.X; x -= 40 {
			sample += fmt.Sprintf(" x=%d:%v", x, rgb(img, x, midY))
		}
		t.Fatalf("could not locate the panel against the background (left=%d, right=%d, width=%d); mid-row sample:%s", left, right, b.Dx(), sample)
	}

	// The panel body must NOT be the background — otherwise the scan
	// above found nothing and every later assertion is vacuous.
	if got := rgb(img, left+20, midY); near(got, wantBG, 24) {
		t.Fatalf("panel interior at x=%d is the background colour %v — the panel did not render", left+20, got)
	}

	// The panel's top edge, found in a column safely inside its body.
	top := -1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		if !near(rgb(img, left+20, y), wantBG, 24) {
			top = y
			break
		}
	}
	if top < 0 {
		t.Fatal("could not find the panel's top edge")
	}

	corner := rgb(img, left+1, top+1)
	if !near(corner, wantBG, 90) {
		t.Errorf("top-left corner is %v, want approximately the background %v.\n"+
			"An opaque GtkWindow background is painting the panel's bounding rectangle, "+
			"so the rounded corners of .ingot-panel show window chrome instead of what is behind. "+
			"theme.PanelWindowClass's rule in style.css is what clears it.", corner, wantBG)
	}
}
