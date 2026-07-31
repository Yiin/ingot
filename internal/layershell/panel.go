package layershell

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// defaultHeightFraction caps the panel's height at 72% of the reference
// monitor's height, measured from the demo so the panel never crowds a
// short display. Not a design token in internal/ui/theme because it is
// a layershell-only sizing policy, not a CSS value.
const defaultHeightFraction = 0.72

// Config is the panel's layer-shell surface policy, every field measured
// or decided by experiment against Hyprland 0.56 (see the epic's
// "Wayland behaviour" notes).
type Config struct {
	// Namespace identifies the surface to compositor tooling and config
	// (e.g. Hyprland windowrulev2). Defaults to "ingot-panel".
	Namespace string
	// MarginEdge is the gap in dp between the surface and the right
	// edge it is anchored to.
	MarginEdge int
	// Width is the surface's fixed width in dp.
	Width int
	// MaxHeight caps the surface height in dp.
	MaxHeight int
	// HeightFraction additionally caps the surface height at this
	// fraction of the reference monitor's height.
	HeightFraction float64
}

// DefaultConfig returns the panel policy measured in the epic: overlay
// layer, namespace "ingot-panel", 360dp wide, up to 640dp tall capped at
// 72% of the monitor height, 12dp from the right edge.
func DefaultConfig() Config {
	return Config{
		Namespace:      "ingot-panel",
		MarginEdge:     theme.PanelMarginEdge,
		Width:          theme.PanelWidth,
		MaxHeight:      theme.PanelHeight,
		HeightFraction: defaultHeightFraction,
	}
}

// Panel turns a *gtk.Window into a right-docked overlay layer-shell
// surface and owns its map/unmap lifecycle. It never touches the
// window's content — that is internal/ui's job; Panel only owns the
// surface.
type Panel struct {
	win *gtk.Window
	cfg Config
}

// New configures win as a panel surface per cfg and shows it map-ready:
//
//   - IsSupported is checked first. gtk4-layer-shell can be linked and
//     still find no compositor support (X11, GNOME-on-Wayland, or any
//     Wayland compositor that never advertises zwlr_layer_shell_v1); New
//     returns an error in that case instead of silently falling back to
//     an ordinary window.
//   - Overlay layer, anchored to the right edge only — an axis with no
//     anchor is centred by the wlr-layer-shell convention, which is how
//     the panel ends up vertically centred with no extra math.
//   - Exclusive zone 0, so the panel never pushes other windows aside.
//   - keyboard-mode ON_DEMAND. ON_DEMAND and EXCLUSIVE are
//     indistinguishable on Hyprland (it grabs focus on commit for both),
//     but ON_DEMAND is still the right choice: EXCLUSIVE refuses to
//     yield focus when the user clicks another window.
//   - The surface size is set via SetDefaultSize before New returns, and
//     therefore before win can have been mapped by the caller. Without
//     this, gtk4-layer-shell commits a 200x200 first frame before GTK's
//     real configure roundtrip arrives, visible as a flash.
//
// win must not have been shown yet. ref, if non-nil, is the monitor New
// sizes the surface against; pass nil to size against the display's
// first monitor (Wayland gives clients no way to learn which output is
// focused, so this is a best-effort hint for the initial frame only —
// pin a specific monitor with SetMonitorByConnector for a real
// guarantee).
func New(win *gtk.Window, cfg Config, ref *gdk.Monitor) (*Panel, error) {
	if !IsSupported() {
		return nil, fmt.Errorf("layershell: compositor does not support the wlr-layer-shell protocol")
	}
	if cfg.Namespace == "" {
		cfg.Namespace = "ingot-panel"
	}
	if cfg.HeightFraction <= 0 {
		// A zero fraction would cap panelHeight at 0, and per the
		// header a 0 height passed to SetDefaultSize means "use the
		// default for that axis" — exactly the 200x200 first-frame
		// flash this function exists to prevent.
		cfg.HeightFraction = defaultHeightFraction
	}

	p := &Panel{win: win, cfg: cfg}

	InitForWindow(win)
	SetNamespace(win, cfg.Namespace)
	SetLayer(win, LayerOverlay)
	SetAnchor(win, EdgeRight, true)
	SetMargin(win, EdgeRight, cfg.MarginEdge)
	SetExclusiveZone(win, 0)
	SetKeyboardMode(win, KeyboardModeOnDemand)

	if ref == nil {
		ref = firstMonitor(win)
	}
	SetMonitor(win, ref)
	win.SetDefaultSize(cfg.Width, panelHeight(monitorHeight(ref), cfg))

	return p, nil
}

// Show maps the surface. Per the epic's measured behaviour, mapping
// hands keyboard focus to win while it is up; do this after reading any
// pending PRIMARY selection, or Ingot's own entry claims it.
func (p *Panel) Show() {
	p.win.SetVisible(true)
}

// Hide unmaps the surface. Focus returns to whatever toplevel was
// previously focused automatically — measured behaviour, so Hide
// implements no focus save/restore. Never toggle keyboard-mode instead
// of unmapping: that path is Hyprland issue #8293, where keyboard input
// ends up eaten by neither surface.
func (p *Panel) Hide() {
	p.win.SetVisible(false)
}

// SetMonitorByConnector pins the surface to the monitor whose connector
// name matches name (e.g. "DP-9", matched against gdk.Monitor.Connector
// — verified to join cleanly with the names hyprctl monitors reports),
// and resizes it against that monitor's height. It returns an error if
// no attached monitor has that connector name.
func (p *Panel) SetMonitorByConnector(name string) error {
	display := p.win.Widget.Display()
	monitors := display.Monitors()
	for i := uint(0); i < monitors.NItems(); i++ {
		obj := monitors.Item(i)
		if obj == nil {
			continue
		}
		monitor := &gdk.Monitor{Object: obj}
		if monitor.Connector() == name {
			SetMonitor(p.win, monitor)
			p.win.SetDefaultSize(p.cfg.Width, panelHeight(monitorHeight(monitor), p.cfg))
			return nil
		}
	}
	return fmt.Errorf("layershell: no attached monitor with connector %q", name)
}

// firstMonitor returns win's display's first monitor, or nil if the
// display has none. Used only as a best-effort sizing reference before
// the compositor has placed the surface — see New's doc comment.
func firstMonitor(win *gtk.Window) *gdk.Monitor {
	monitors := win.Widget.Display().Monitors()
	if monitors.NItems() == 0 {
		return nil
	}
	obj := monitors.Item(0)
	if obj == nil {
		return nil
	}
	return &gdk.Monitor{Object: obj}
}

// monitorHeight returns m's geometry height in dp, or 0 if m is nil.
func monitorHeight(m *gdk.Monitor) int {
	if m == nil {
		return 0
	}
	return m.Geometry().Height()
}

// panelHeight returns the surface height in dp: capped at cfg.MaxHeight
// and, when monitorHeightDp is known, additionally capped at
// cfg.HeightFraction of it. A non-positive monitorHeightDp (no monitor
// reference available) skips the second cap.
func panelHeight(monitorHeightDp int, cfg Config) int {
	h := cfg.MaxHeight
	if monitorHeightDp > 0 {
		if capped := int(float64(monitorHeightDp) * cfg.HeightFraction); capped < h {
			h = capped
		}
	}
	return h
}
