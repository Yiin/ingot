package wl

import (
	"context"
	"testing"
)

func TestProbe_DegradesWhenSocketUnreachable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("WAYLAND_DISPLAY", "does-not-exist")
	t.Setenv("XDG_CURRENT_DESKTOP", "Hyprland")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "abc123")

	caps, err := Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe returned an error instead of degrading: %v", err)
	}

	if caps.ExtDataControlManager.Present() || caps.WlrDataControlManager.Present() ||
		caps.WlrLayerShell.Present() || caps.VirtualKeyboardManager.Present() {
		t.Errorf("Probe reported a present interface with no reachable compositor: %+v", caps)
	}

	// Environment identity must still be reported even though the
	// registry roundtrip failed.
	if caps.Desktop != "Hyprland" {
		t.Errorf("Desktop = %q, want %q", caps.Desktop, "Hyprland")
	}
	if !caps.Hyprland {
		t.Error("Hyprland = false, want true from HYPRLAND_INSTANCE_SIGNATURE being set")
	}
}

func TestProbe_DegradesWhenRuntimeDirUnset(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")

	caps, err := Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe returned an error instead of degrading: %v", err)
	}
	if caps.WlrLayerShell.Present() {
		t.Errorf("WlrLayerShell present with no XDG_RUNTIME_DIR: %+v", caps)
	}
}

func TestProbe_AlreadyCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Probe(ctx)
	if err == nil {
		t.Fatal("Probe(already-canceled ctx) returned nil error")
	}
}

func TestInterface_Present(t *testing.T) {
	if (Interface{}).Present() {
		t.Error("zero-value Interface reports Present() == true")
	}
	if !(Interface{Version: 1}).Present() {
		t.Error("Interface{Version: 1}.Present() == false")
	}
}
