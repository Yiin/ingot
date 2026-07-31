package toast

import (
	"fmt"
	"log/slog"

	"github.com/diamondburned/gotk4/pkg/glib/v2"

	"github.com/Yiin/ingot/internal/layershell"
)

// layerShellSupported is a seam over layershell.IsSupported so a test can
// force New down the org.freedesktop.Notifications fallback path without
// needing a compositor that genuinely lacks wlr-layer-shell — see
// toast_integration_test.go's TestNew_FallsBackToNotificationsWhenLayerShellUnsupported,
// the acceptance criterion "the notification fallback is exercised with
// layer-shell reported unsupported."
var layerShellSupported = layershell.IsSupported

// hudShower is what Toaster.Captured needs from whichever backend New
// picked: HUD when the compositor supports wlr-layer-shell, or
// fallbackNotifier when it does not.
type hudShower interface {
	Show(text string)
}

// Toaster is the concrete Notifier: Captured goes to whichever hudShower
// New picked, Message always goes to the light in-panel toast, which is
// an ordinary widget and needs no layer-shell surface of its own.
//
// Both methods are safe to call from any goroutine, not just the GTK
// main thread: they hop over via glib.IdleAdd before touching any widget
// or D-Bus state, the same pattern this codebase's other GTK-adjacent
// packages use to bridge a background goroutine (e.g. the evdev capture
// reader that will end up calling Captured) onto the single GTK thread.
type Toaster struct {
	captured hudShower
	panel    *InPanel
}

// New builds the app's Notifier. It tries the dark HUD's own layer-shell
// surface first; only when that is unsupported, or fails to build, does
// it fall back to org.freedesktop.Notifications — see fallbackNotifier's
// doc comment for why that fallback must never be the default path.
func New(logger *slog.Logger) (*Toaster, error) {
	panel := NewInPanel()

	var hudErr error
	if layerShellSupported() {
		hud, err := NewHUD()
		if err == nil {
			return &Toaster{captured: hud, panel: panel}, nil
		}
		hudErr = err
	} else {
		hudErr = fmt.Errorf("compositor does not support wlr-layer-shell")
	}

	fb, fbErr := newFallbackNotifier(logger)
	if fbErr != nil {
		return nil, fmt.Errorf("toast: no layer-shell (%v) and no notification fallback (%w)", hudErr, fbErr)
	}
	return &Toaster{captured: fb, panel: panel}, nil
}

// Panel returns the light in-panel toast, to embed as a GtkOverlay child
// of the panel (copper-l2z.26).
func (t *Toaster) Panel() *InPanel { return t.panel }

func (t *Toaster) Captured(text string) {
	glib.IdleAdd(func() bool {
		t.captured.Show(text)
		return false
	})
}

func (t *Toaster) Message(text string) {
	glib.IdleAdd(func() bool {
		t.panel.Show(text)
		return false
	})
}

var _ Notifier = (*Toaster)(nil)
