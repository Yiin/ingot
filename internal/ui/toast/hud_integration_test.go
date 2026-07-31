//go:build integration

// These tests need a real GDK display connection (gtk.Init +
// gdk.DisplayGetDefault) and a compositor that actually speaks
// zwlr_layer_shell_v1 — same convention as
// internal/layershell/panel_integration_test.go, and need copper-l2z.31's
// headless sway harness (WLR_BACKENDS=headless, GSK_RENDERER=cairo) to
// actually run. This worktree has no display at all: these tests only
// need to compile here (go vet -tags integration), never execute.
//
// TestHUD_ShowStealsNoFocus is the closest this package gets to the
// acceptance criterion's "asserted with a sink window receiving
// keystrokes throughout": it uses a second real GTK toplevel as the sink
// and polls IsActive() through the HUD's full show/hold/hide cycle,
// pumping GLib's default main context by hand since no gtk.Main() loop
// is running in a test binary — the same sink-window idiom
// panel_integration_test.go uses for its own focus-return test.
package toast

import (
	"sync"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/layershell"
)

var gtkInitOnce sync.Once

func requireDisplay(t *testing.T) *gdk.Display {
	t.Helper()
	gtkInitOnce.Do(gtk.Init)
	display := gdk.DisplayGetDefault()
	if display == nil {
		t.Skip("no GDK display available")
	}
	return display
}

func requireLayerShell(t *testing.T) {
	t.Helper()
	if !layershell.IsSupported() {
		t.Skip("compositor does not support wlr-layer-shell")
	}
}

// pumpMainLoop drains pending GLib main-context events for at least d,
// the only way glib.TimeoutAdd callbacks (sequencer/HUD's fade timers)
// actually fire in a test binary with no gtk.Main() running.
func pumpMainLoop(d time.Duration) {
	ctx := glib.MainContextDefault()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		for ctx.Pending() {
			ctx.Iteration(false)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestNewHUD_RejectsAnUnsupportedCompositor(t *testing.T) {
	requireDisplay(t)
	if layershell.IsSupported() {
		t.Skip("this harness's compositor supports wlr-layer-shell; nothing to reject")
	}

	if _, err := NewHUD(); err == nil {
		t.Fatal("NewHUD returned a nil error against a compositor with no wlr-layer-shell support")
	}
}

func TestHUD_ShowMapsThenAutoHidesAfterHoldPlusFadeOut(t *testing.T) {
	requireDisplay(t)
	requireLayerShell(t)

	hud, err := NewHUD()
	if err != nil {
		t.Fatalf("NewHUD: %v", err)
	}

	hud.Show("Captured: test")
	if !hud.win.Visible() {
		t.Fatal("HUD surface not mapped immediately after Show")
	}

	pumpMainLoop(HoldDuration + FadeOutDuration + 300*time.Millisecond)

	if hud.win.Visible() {
		t.Error("HUD surface still mapped after hold+fade-out elapsed")
	}
}

// TestHUD_ShowStealsNoFocus covers the acceptance criterion directly:
// with keyboard-mode NONE, a sink toplevel active before Show must stay
// active before, during, and after the HUD's surface is mapped.
func TestHUD_ShowStealsNoFocus(t *testing.T) {
	requireDisplay(t)
	requireLayerShell(t)

	sink := gtk.NewWindow()
	sink.SetTitle("toast-hud-test-sink")
	sink.SetVisible(true)
	pumpMainLoop(50 * time.Millisecond)

	if !sink.IsActive() {
		t.Skip("sink window never activated in this harness; cannot assert it keeps focus")
	}

	hud, err := NewHUD()
	if err != nil {
		t.Fatalf("NewHUD: %v", err)
	}

	hud.Show("Captured: test")
	if !sink.IsActive() {
		t.Fatal("sink lost activation the instant the HUD was shown")
	}

	deadline := time.Now().Add(HoldDuration + FadeOutDuration + 300*time.Millisecond)
	ctx := glib.MainContextDefault()
	for time.Now().Before(deadline) {
		for ctx.Pending() {
			ctx.Iteration(false)
		}
		if !sink.IsActive() {
			t.Fatal("sink lost activation while the HUD was mapped")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestHUD_SecondShowWithinHoldNeverHidesEarly is the GTK-level analogue
// of TestSequencer_SecondShowWithinHold_ReplacesRatherThanStacks: a
// second Show partway through the hold must keep the surface mapped past
// the first Show's original hold deadline.
func TestHUD_SecondShowWithinHoldNeverHidesEarly(t *testing.T) {
	requireDisplay(t)
	requireLayerShell(t)

	hud, err := NewHUD()
	if err != nil {
		t.Fatalf("NewHUD: %v", err)
	}

	hud.Show("Captured: first")
	pumpMainLoop(HoldDuration - 200*time.Millisecond)
	hud.Show("Captured: second")

	// The first Show's hold would have elapsed by now; the replace must
	// have reset it.
	pumpMainLoop(300 * time.Millisecond)
	if !hud.win.Visible() {
		t.Fatal("HUD hid before the second Show's own hold window elapsed — toasts stacked/raced instead of replacing")
	}
}

// TestHUD_ShowDuringFadeOutCancelsTheStaleHideTimer covers the bug a
// Fable review caught before this landed: a Show that arrives after the
// hold has already elapsed (sequencer treats it as a fresh show, since
// its own visible flag flips to false the instant the hold timer fires)
// re-maps the surface in enter — but exit's own raw fade-out-then-hide
// glib.TimeoutAdd from the previous cycle was never canceled, and would
// otherwise still fire afterwards and unmap the just-re-shown toast.
func TestHUD_ShowDuringFadeOutCancelsTheStaleHideTimer(t *testing.T) {
	requireDisplay(t)
	requireLayerShell(t)

	hud, err := NewHUD()
	if err != nil {
		t.Fatalf("NewHUD: %v", err)
	}

	hud.Show("Captured: first")
	// Let the hold elapse so exit() arms its own fade-out-then-hide
	// timer, then show again partway through that fade-out.
	pumpMainLoop(HoldDuration + FadeOutDuration/2)
	hud.Show("Captured: second")

	// The stale timer from the first cycle would fire somewhere around
	// now if it survived; give it well past that point and assert the
	// surface is still mapped.
	pumpMainLoop(FadeOutDuration + 200*time.Millisecond)
	if !hud.win.Visible() {
		t.Fatal("HUD surface was unmapped by a stale fade-out timer from before the second Show")
	}
}
