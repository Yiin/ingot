package theme

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// PanelWindowClass is the CSS class the panel's GtkWindow must carry for
// the stylesheet to clear its background. GTK paints every window an
// opaque theme colour, and under layer-shell the surface is exactly the
// panel's size, so that square shows at the four corners .ingot-panel
// rounds off. The name lives here, beside the rule that consumes it, so
// the window and the stylesheet cannot drift apart.
const PanelWindowClass = "ingot-panel-window"

// Load registers the bundled Inter Variable font and installs the panel
// stylesheet on display at application priority, so every widget created
// afterwards picks it up.
//
// Call this once, right after gtk.Init(), before building any widget —
// gtkapp.Run (copper-l2z.16) is responsible for that ordering.
func Load(display *gdk.Display) error {
	if err := registerBundledFont(); err != nil {
		return err
	}

	if display == nil {
		return fmt.Errorf("theme: nil display")
	}

	provider := gtk.NewCSSProvider()
	// Deliberately not calling provider.ConnectParsingError: gotk4 v0.4.0
	// double-frees the borrowed GError at gtk_export.go:3948 and the
	// process aborts with "free(): invalid pointer". GTK already emits
	// every parse error as a WARN "Theme parser error" line through
	// gotk4's slog bridge (glib.LogSetWriterFunc, gotk4/pkg/glib/v2) —
	// that is the signal this package's tests assert on instead.
	provider.LoadFromString(CSS)
	gtk.StyleContextAddProviderForDisplay(display, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

	return nil
}
