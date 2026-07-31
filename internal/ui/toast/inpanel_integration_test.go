//go:build integration

// See hud_integration_test.go's header: same headless-sway-only, compile-
// only-here convention. InPanel is an ordinary widget rather than its
// own surface, so these tests need a display but not layer-shell
// support — a plain gtk.Init suffices, no requireLayerShell.
package toast

import (
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// TestInPanel_CentreXEqualsPanelCentre covers the acceptance criterion's
// "the light variant's centre x equals the panel centre": InPanel's
// revealer is centred within whatever GtkOverlay it is added to, so as
// long as that overlay spans the full panel width (copper-l2z.26's job),
// HAlignCenter is sufficient — this only proves InPanel holds up its half
// of that contract.
func TestInPanel_CentreXEqualsPanelCentre(t *testing.T) {
	requireDisplay(t)

	p := NewInPanel()
	if got := p.Widget().HAlign(); got != gtk.AlignCenter {
		t.Errorf("revealer HAlign = %v, want AlignCenter", got)
	}
}

func TestInPanel_ShowRevealsThenAutoHidesAfterHoldPlusFadeOut(t *testing.T) {
	requireDisplay(t)

	p := NewInPanel()
	p.Show("Copied as List")
	if !p.Widget().RevealChild() {
		t.Fatal("RevealChild() false immediately after Show")
	}

	pumpMainLoop(HoldDuration + FadeOutDuration + 300*time.Millisecond)

	if p.Widget().RevealChild() {
		t.Error("RevealChild() still true after hold+fade-out elapsed")
	}
}

// TestInPanel_SecondShowWithinHoldNeverHidesEarly mirrors
// TestHUD_SecondShowWithinHoldNeverHidesEarly for the light toast.
func TestInPanel_SecondShowWithinHoldNeverHidesEarly(t *testing.T) {
	requireDisplay(t)

	p := NewInPanel()
	p.Show("Copied as List")
	pumpMainLoop(HoldDuration - 200*time.Millisecond)
	p.Show("Copied as List")

	pumpMainLoop(300 * time.Millisecond)
	if !p.Widget().RevealChild() {
		t.Fatal("light toast hid before the second Show's own hold window elapsed — toasts stacked/raced instead of replacing")
	}
}

// TestInPanel_ShowDuringFadeOutCancelsTheStaleHideTimer mirrors
// hud_integration_test.go's TestHUD_ShowDuringFadeOutCancelsTheStaleHideTimer
// for the light toast's own RevealChild-based hide timer.
func TestInPanel_ShowDuringFadeOutCancelsTheStaleHideTimer(t *testing.T) {
	requireDisplay(t)

	p := NewInPanel()
	p.Show("Copied as List")
	pumpMainLoop(HoldDuration + FadeOutDuration/2)
	p.Show("Copied as List")

	pumpMainLoop(FadeOutDuration + 200*time.Millisecond)
	if !p.Widget().RevealChild() {
		t.Fatal("light toast was hidden by a stale fade-out timer from before the second Show")
	}
}

func TestInPanel_SetBottomInset(t *testing.T) {
	requireDisplay(t)

	p := NewInPanel()
	p.SetBottomInset(120)
	if got := p.Widget().MarginBottom(); got != 120 {
		t.Errorf("MarginBottom() = %d, want 120 after SetBottomInset(120)", got)
	}
}
