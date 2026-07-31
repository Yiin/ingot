package wl

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"
)

// defaultProbeTimeout bounds one registry roundtrip against a compositor
// that accepts the connection but never replies, so a stuck or broken
// compositor degrades Probe instead of hanging it indefinitely.
const defaultProbeTimeout = 2 * time.Second

// dial connects to the compositor named by WAYLAND_DISPLAY, resolved
// against XDG_RUNTIME_DIR per the Wayland socket discovery rule: an
// absolute WAYLAND_DISPLAY is used as-is, otherwise it is joined to
// XDG_RUNTIME_DIR; "wayland-0" is the documented fallback display name
// when WAYLAND_DISPLAY is unset.
func dial(ctx context.Context) (net.Conn, error) {
	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		display = "wayland-0"
	}

	path := display
	if !filepath.IsAbs(path) {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			return nil, errors.New("wl: XDG_RUNTIME_DIR not set")
		}
		path = filepath.Join(runtimeDir, display)
	}

	d := net.Dialer{Timeout: defaultProbeTimeout}
	return d.DialContext(ctx, "unix", path)
}
