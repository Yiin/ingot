package toast

import (
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"
)

const (
	notifyBusName    = "org.freedesktop.Notifications"
	notifyObjectPath = dbus.ObjectPath("/org/freedesktop/Notifications")
	notifyMethod     = notifyBusName + ".Notify"
)

// fallbackNotifier speaks org.freedesktop.Notifications directly (measured
// 5ms via mako) — used only when layer-shell is unavailable (see New in
// toast.go). It loses to the dark HUD on three counts and must never be
// the default: a notification daemon may not exist; the daemon owns the
// appearance, so Ingot cannot match its own design or place the toast;
// and toasts pile into the user's notification history instead of
// disappearing on their own.
type fallbackNotifier struct {
	conn   *dbus.Conn
	logger *slog.Logger
}

// newFallbackNotifier connects to the session bus — org.freedesktop.
// Notifications is a per-session service, unlike internal/session's
// system-bus logind watch.
func newFallbackNotifier(logger *slog.Logger) (*fallbackNotifier, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("toast: connect session bus: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &fallbackNotifier{conn: conn, logger: logger}, nil
}

// Show fires a best-effort desktop notification carrying text. Errors are
// logged, not returned or panicked on: a missing/unreachable notification
// daemon is exactly the degraded case this fallback exists to tolerate,
// not a reason to crash the capture flow that triggered it.
func (f *fallbackNotifier) Show(text string) {
	obj := f.conn.Object(notifyBusName, notifyObjectPath)
	args := notifyArgs(text)
	call := obj.Call(notifyMethod, 0, args...)
	if call.Err != nil {
		f.logger.Warn("toast: notification fallback failed", "err", call.Err)
	}
}

// Close closes the session bus connection.
func (f *fallbackNotifier) Close() error { return f.conn.Close() }

// notifyArgs builds org.freedesktop.Notifications.Notify's argument list
// for text, factored out of Show so the exact shape sent over the bus is
// testable without a live session bus or daemon.
func notifyArgs(text string) []any {
	return []any{
		"Ingot",                   // app_name
		uint32(0),                 // replaces_id: 0 = always a new notification
		"",                        // app_icon
		text,                      // summary
		"",                        // body
		[]string{},                // actions
		map[string]dbus.Variant{}, // hints
		int32(-1),                 // expire_timeout: -1 = the daemon's own default
	}
}

var _ hudShower = (*fallbackNotifier)(nil)
