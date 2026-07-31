//go:build integration

// See hud_integration_test.go's header: same headless-sway-only, compile-
// only-here convention.
package toast

import "testing"

// TestNew_FallsBackToNotificationsWhenLayerShellUnsupported exercises the
// acceptance criterion "the notification fallback is exercised with
// layer-shell reported unsupported" — via the layerShellSupported seam
// (toast.go) rather than an actual compositor with no wlr-layer-shell
// support, since every harness this repo runs against does have it.
func TestNew_FallsBackToNotificationsWhenLayerShellUnsupported(t *testing.T) {
	requireDisplay(t)

	orig := layerShellSupported
	layerShellSupported = func() bool { return false }
	defer func() { layerShellSupported = orig }()

	toaster, err := New(nil)
	if err != nil {
		t.Skipf("no session bus reachable to build the fallback notifier: %v", err)
	}
	defer func() {
		if fb, ok := toaster.captured.(*fallbackNotifier); ok {
			_ = fb.Close()
		}
	}()

	if _, ok := toaster.captured.(*fallbackNotifier); !ok {
		t.Errorf("captured = %T, want *fallbackNotifier when layer-shell is reported unsupported", toaster.captured)
	}
}
