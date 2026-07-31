//go:build integration

// These tests need a real GDK display connection (gtk.Init +
// gdk.DisplayGetDefault) and, for TestNew*, a compositor that actually
// speaks zwlr_layer_shell_v1 — so they are gated behind the integration
// tag, same convention as internal/ui/theme/display_test.go and
// internal/ui/notelist/list_integration_test.go, and need copper-l2z.31's
// headless sway harness (WLR_BACKENDS=headless, GSK_RENDERER=cairo) to
// actually run. Do not run this file against a live desktop session:
// unlike the other internal/ui integration tests, a layer-shell surface
// this creates would actually be mapped as an overlay. This worktree has
// no display at all — these tests only need to compile here
// (go vet -tags integration), never execute.
//
// TestFocusReturnsToPreviousToplevelOnHide is the closest this package
// gets to the acceptance criterion's "asserted with a scripted sink
// window": it uses a second real GTK toplevel as the sink instead of an
// external terminal, since scripting an actual terminal process belongs
// to copper-l2z.31's harness, not a Go test binary.
package layershell

import (
	"sync"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

var gtkInitOnce sync.Once

// requireDisplay initialises GTK once per test binary and returns the
// default display, skipping the test if none is reachable.
func requireDisplay(t *testing.T) *gdk.Display {
	t.Helper()
	gtkInitOnce.Do(gtk.Init)
	display := gdk.DisplayGetDefault()
	if display == nil {
		t.Skip("no GDK display available")
	}
	return display
}

// requireLayerShell skips the test when the compositor does not speak
// zwlr_layer_shell_v1 (e.g. a plain X11 or GNOME session).
func requireLayerShell(t *testing.T) {
	t.Helper()
	if !IsSupported() {
		t.Skip("compositor does not support wlr-layer-shell")
	}
}

func TestIsSupported_DoesNotPanicWithADisplay(t *testing.T) {
	requireDisplay(t)
	// Either answer is valid depending on the harness's compositor; the
	// point of this test is that calling it at all is safe.
	_ = IsSupported()
}

func TestNew_RejectsAnUnsupportedCompositor(t *testing.T) {
	requireDisplay(t)
	if IsSupported() {
		t.Skip("this harness's compositor supports wlr-layer-shell; nothing to reject")
	}

	win := gtk.NewWindow()
	if _, err := New(win, DefaultConfig(), nil); err == nil {
		t.Fatal("New returned a nil error against a compositor with no wlr-layer-shell support")
	}
}

func TestNew_SetsSizeBeforeFirstShow(t *testing.T) {
	requireDisplay(t)
	requireLayerShell(t)

	win := gtk.NewWindow()
	cfg := DefaultConfig()
	if _, err := New(win, cfg, nil); err != nil {
		t.Fatalf("New: %v", err)
	}

	width, height := win.DefaultSize()
	if width != cfg.Width {
		t.Errorf("default width = %d, want %d (a zero or 200x200 default means the first frame flashes)", width, cfg.Width)
	}
	if height <= 0 {
		t.Errorf("default height = %d, want > 0 before the surface is ever shown", height)
	}
}

// TestNew_DefaultsAZeroHeightFraction covers a hand-built Config that
// forgets HeightFraction: New must not pass SetDefaultSize a height of
// 0, since gtk4-layer-shell treats 0 as "use the default for that axis"
// — the exact 200x200 flash this package exists to prevent.
func TestNew_DefaultsAZeroHeightFraction(t *testing.T) {
	requireDisplay(t)
	requireLayerShell(t)

	win := gtk.NewWindow()
	cfg := Config{Namespace: "ingot-panel-test", MarginEdge: 12, Width: 360, MaxHeight: 640}
	if _, err := New(win, cfg, nil); err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, height := win.DefaultSize(); height <= 0 {
		t.Errorf("default height = %d, want > 0 with HeightFraction left at its zero value", height)
	}
}

func TestShowHide_TogglesVisibility(t *testing.T) {
	requireDisplay(t)
	requireLayerShell(t)

	win := gtk.NewWindow()
	p, err := New(win, DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	p.Show()
	if !win.Visible() {
		t.Error("Show did not map the surface (Visible() == false)")
	}

	p.Hide()
	if win.Visible() {
		t.Error("Hide did not unmap the surface (Visible() == true)")
	}
}

// TestFocusReturnsToPreviousToplevelOnHide covers the acceptance
// criterion's map/unmap focus behaviour: with a sink toplevel active,
// showing then hiding the panel must hand focus back to the sink without
// the panel ever calling any focus save/restore API — see Panel.Hide's
// doc comment.
func TestFocusReturnsToPreviousToplevelOnHide(t *testing.T) {
	requireDisplay(t)
	requireLayerShell(t)

	sink := gtk.NewWindow()
	sink.SetTitle("layershell-test-sink")
	sink.SetVisible(true)

	win := gtk.NewWindow()
	p, err := New(win, DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	p.Show()
	p.Hide()

	if !sink.IsActive() {
		t.Error("sink window did not regain focus after the panel was hidden")
	}
}

func TestSetMonitorByConnector_UnknownNameReturnsAnError(t *testing.T) {
	requireDisplay(t)
	requireLayerShell(t)

	win := gtk.NewWindow()
	p, err := New(win, DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.SetMonitorByConnector("definitely-not-a-real-connector"); err == nil {
		t.Fatal("SetMonitorByConnector returned a nil error for an unknown connector name")
	}
}

func TestSetMonitorByConnector_MatchesAnAttachedMonitor(t *testing.T) {
	display := requireDisplay(t)
	requireLayerShell(t)

	monitors := display.Monitors()
	if monitors.NItems() == 0 {
		t.Skip("no attached monitors reported")
	}
	obj := monitors.Item(0)
	if obj == nil {
		t.Skip("first monitor slot was nil")
	}
	want := (&gdk.Monitor{Object: obj}).Connector()
	if want == "" {
		t.Skip("first monitor reports no connector name")
	}

	win := gtk.NewWindow()
	p, err := New(win, DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.SetMonitorByConnector(want); err != nil {
		t.Fatalf("SetMonitorByConnector(%q): %v", want, err)
	}
}
