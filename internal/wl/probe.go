package wl

import (
	"context"
	"os"
)

// The four interface names this package looks for, per the epic's known
// compositor matrix.
const (
	extDataControlManagerInterface  = "ext_data_control_manager_v1"
	wlrDataControlManagerInterface  = "zwlr_data_control_manager_v1"
	wlrLayerShellInterface          = "zwlr_layer_shell_v1"
	virtualKeyboardManagerInterface = "zwp_virtual_keyboard_manager_v1"
)

// Interface describes whether the compositor advertised one Wayland
// global, and at which version.
type Interface struct {
	Version uint32
}

// Present reports whether the compositor advertised this interface at
// all. A zero Version means absent: version 0 is not a valid Wayland
// interface version, so it is a safe sentinel for "not seen."
func (i Interface) Present() bool { return i.Version > 0 }

// Capabilities is the result of one wl_registry roundtrip plus a read of
// two identifying environment variables.
type Capabilities struct {
	ExtDataControlManager  Interface
	WlrDataControlManager  Interface
	WlrLayerShell          Interface
	VirtualKeyboardManager Interface

	// Desktop is XDG_CURRENT_DESKTOP verbatim, e.g. "Hyprland" or "KDE".
	Desktop string
	// Hyprland reports whether HYPRLAND_INSTANCE_SIGNATURE is set, which
	// is a more specific and reliable Hyprland signal than Desktop, since
	// some setups list Hyprland alongside other tokens in
	// XDG_CURRENT_DESKTOP.
	Hyprland bool
}

// Probe performs one wl_registry roundtrip against the compositor named
// by WAYLAND_DISPLAY and reports which of the four interfaces Ingot
// depends on are present, and at what version, alongside compositor
// identity read from the environment.
//
// Probe never fails outright: if the socket cannot be reached, the
// compositor speaks a broken protocol, or ctx expires mid-roundtrip, it
// returns a Capabilities with every interface absent instead of an
// error — the acceptance criterion is that a probe failure degrades to
// an unknown capability set, since capability probing gates optional
// features and must never block startup. The error return is reserved
// for a ctx that is already canceled or expired when Probe is called.
func Probe(ctx context.Context) (Capabilities, error) {
	caps := Capabilities{
		Desktop:  os.Getenv("XDG_CURRENT_DESKTOP"),
		Hyprland: os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "",
	}

	if err := ctx.Err(); err != nil {
		return caps, err
	}

	conn, err := dial(ctx)
	if err != nil {
		return caps, nil
	}
	defer func() { _ = conn.Close() }()

	globals, err := fetchGlobals(ctx, conn)
	if err != nil {
		return caps, nil
	}

	for _, g := range globals {
		switch g.Interface {
		case extDataControlManagerInterface:
			caps.ExtDataControlManager = Interface{Version: g.Version}
		case wlrDataControlManagerInterface:
			caps.WlrDataControlManager = Interface{Version: g.Version}
		case wlrLayerShellInterface:
			caps.WlrLayerShell = Interface{Version: g.Version}
		case virtualKeyboardManagerInterface:
			caps.VirtualKeyboardManager = Interface{Version: g.Version}
		}
	}

	return caps, nil
}
