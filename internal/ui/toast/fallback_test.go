package toast

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

// TestNotifyArgs pins the exact org.freedesktop.Notifications.Notify
// argument shape (app_name, replaces_id, app_icon, summary, body,
// actions, hints, expire_timeout) sent by the layer-shell-unavailable
// fallback, without needing a live session bus or notification daemon —
// the seam this package's own tests can actually exercise; the real
// round-trip needs copper-l2z.31's headless harness (or a real mako),
// same precedent as every other display/bus-dependent test in this repo.
func TestNotifyArgs(t *testing.T) {
	args := notifyArgs("Captured: hello world")

	if len(args) != 8 {
		t.Fatalf("notifyArgs returned %d args, want 8", len(args))
	}

	appName, ok := args[0].(string)
	if !ok || appName != "Ingot" {
		t.Errorf("app_name = %#v, want %q", args[0], "Ingot")
	}

	replacesID, ok := args[1].(uint32)
	if !ok || replacesID != 0 {
		t.Errorf("replaces_id = %#v, want uint32(0) (never replace a prior toast in the daemon's own history)", args[1])
	}

	if icon, ok := args[2].(string); !ok || icon != "" {
		t.Errorf("app_icon = %#v, want \"\"", args[2])
	}

	summary, ok := args[3].(string)
	if !ok || summary != "Captured: hello world" {
		t.Errorf("summary = %#v, want the text passed in", args[3])
	}

	if body, ok := args[4].(string); !ok || body != "" {
		t.Errorf("body = %#v, want \"\"", args[4])
	}

	actions, ok := args[5].([]string)
	if !ok || len(actions) != 0 {
		t.Errorf("actions = %#v, want an empty []string", args[5])
	}

	hints, ok := args[6].(map[string]dbus.Variant)
	if !ok || len(hints) != 0 {
		t.Errorf("hints = %#v, want an empty map[string]dbus.Variant", args[6])
	}

	expire, ok := args[7].(int32)
	if !ok || expire != -1 {
		t.Errorf("expire_timeout = %#v, want int32(-1) (the daemon's own default)", args[7])
	}
}
