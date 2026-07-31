package setup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yiin/ingot/internal/store/paths"
	"github.com/Yiin/ingot/internal/wl"
)

func TestWaylandSessionCheck(t *testing.T) {
	c := waylandSessionCheck{}

	t.Setenv("WAYLAND_DISPLAY", "")
	if r := c.Run(context.Background()); r.Severity != Fatal {
		t.Errorf("Run() with no WAYLAND_DISPLAY: Severity = %v, want Fatal", r.Severity)
	}

	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	if r := c.Run(context.Background()); r.Severity != OK {
		t.Errorf("Run() with WAYLAND_DISPLAY set: Severity = %v, want OK", r.Severity)
	}
}

func TestLayerShellCheck(t *testing.T) {
	present := layerShellCheck{probe: func(context.Context) (wl.Capabilities, error) {
		return wl.Capabilities{WlrLayerShell: wl.Interface{Version: 4}}, nil
	}}
	if r := present.Run(context.Background()); r.Severity != OK {
		t.Errorf("Run() with layer-shell present: Severity = %v, want OK", r.Severity)
	}

	absent := layerShellCheck{probe: func(context.Context) (wl.Capabilities, error) {
		return wl.Capabilities{}, nil
	}}
	if r := absent.Run(context.Background()); r.Severity != Fatal {
		t.Errorf("Run() with layer-shell absent: Severity = %v, want Fatal", r.Severity)
	}

	failing := layerShellCheck{probe: func(context.Context) (wl.Capabilities, error) {
		return wl.Capabilities{}, errors.New("no socket")
	}}
	if r := failing.Run(context.Background()); r.Severity != Fatal {
		t.Errorf("Run() on probe error: Severity = %v, want Fatal", r.Severity)
	}
}

func TestReadableKeyboardsCheck(t *testing.T) {
	tests := []struct {
		name   string
		status KeyboardStatus
		err    error
		want   Severity
	}{
		{name: "healthy", status: KeyboardStatus{Detected: 2, Readable: 2}, want: OK},
		{name: "none detected", status: KeyboardStatus{Detected: 0, Readable: 0}, want: Warn},
		{name: "none readable", status: KeyboardStatus{Detected: 2, Readable: 0}, want: Warn},
		{name: "partially readable", status: KeyboardStatus{Detected: 2, Readable: 1}, want: Warn},
		{name: "probe error", err: errors.New("boom"), want: Warn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := readableKeyboardsCheck{probe: func() (KeyboardStatus, error) { return tt.status, tt.err }}
			if r := c.Run(context.Background()); r.Severity != tt.want {
				t.Errorf("Run() Severity = %v, want %v", r.Severity, tt.want)
			}
		})
	}
}

func TestUdevRuleCheck(t *testing.T) {
	tests := []struct {
		state State
		want  Severity
	}{
		{state: Installed, want: OK},
		{state: Modified, want: Warn},
		{state: NotInstalled, want: Warn},
	}
	for _, tt := range tests {
		c := udevRuleCheck{installer: NewFakeInstaller(tt.state)}
		if r := c.Run(context.Background()); r.Severity != tt.want {
			t.Errorf("Run() with state %v: Severity = %v, want %v", tt.state, r.Severity, tt.want)
		}
	}
}

func TestActiveLocalSessionCheck(t *testing.T) {
	tests := []struct {
		name  string
		props map[string]string
		err   error
		want  Severity
	}{
		{name: "active local", props: map[string]string{"Active": "yes", "Remote": "no"}, want: OK},
		{name: "not active", props: map[string]string{"Active": "no", "Remote": "no"}, want: Warn},
		{name: "remote", props: map[string]string{"Active": "yes", "Remote": "yes"}, want: Warn},
		{name: "probe error", err: errors.New("no bus"), want: Warn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := activeLocalSessionCheck{probe: func(context.Context) (map[string]string, error) { return tt.props, tt.err }}
			if r := c.Run(context.Background()); r.Severity != tt.want {
				t.Errorf("Run() Severity = %v, want %v", r.Severity, tt.want)
			}
		})
	}
}

func TestActiveLocalSession(t *testing.T) {
	active, reason, err := ActiveLocalSession(context.Background(), func(context.Context) (map[string]string, error) {
		return map[string]string{"Active": "yes", "Remote": "no"}, nil
	})
	if err != nil || !active || reason != "" {
		t.Errorf("ActiveLocalSession() = (%v, %q, %v), want (true, \"\", nil)", active, reason, err)
	}

	active, reason, err = ActiveLocalSession(context.Background(), func(context.Context) (map[string]string, error) {
		return map[string]string{"Active": "yes", "Remote": "yes"}, nil
	})
	if err != nil || active || reason == "" {
		t.Errorf("ActiveLocalSession() over a remote session = (%v, %q, %v), want (false, non-empty, nil)", active, reason, err)
	}
}

func TestDataControlCheck(t *testing.T) {
	present := dataControlCheck{probe: func(context.Context) (wl.Capabilities, error) {
		return wl.Capabilities{ExtDataControlManager: wl.Interface{Version: 1}}, nil
	}}
	if r := present.Run(context.Background()); r.Severity != OK {
		t.Errorf("Run() with ext_data_control present: Severity = %v, want OK", r.Severity)
	}

	absent := dataControlCheck{probe: func(context.Context) (wl.Capabilities, error) {
		return wl.Capabilities{}, nil
	}}
	if r := absent.Run(context.Background()); r.Severity != Warn {
		t.Errorf("Run() with neither data-control protocol: Severity = %v, want Warn", r.Severity)
	}
}

func TestClipboardToolsCheck(t *testing.T) {
	found := clipboardToolsCheck{lookPath: func(string) (string, error) { return "/usr/bin/x", nil }}
	if r := found.Run(context.Background()); r.Severity != OK {
		t.Errorf("Run() with both tools present: Severity = %v, want OK", r.Severity)
	}

	missing := clipboardToolsCheck{lookPath: func(string) (string, error) { return "", errors.New("not found") }}
	if r := missing.Run(context.Background()); r.Severity != Warn {
		t.Errorf("Run() with tools missing: Severity = %v, want Warn", r.Severity)
	}
}

func TestWtypeCheck(t *testing.T) {
	found := wtypeCheck{lookPath: func(string) (string, error) { return "/usr/bin/wtype", nil }}
	if r := found.Run(context.Background()); r.Severity != OK {
		t.Errorf("Run() with wtype present: Severity = %v, want OK", r.Severity)
	}

	missing := wtypeCheck{lookPath: func(string) (string, error) { return "", errors.New("not found") }}
	if r := missing.Run(context.Background()); r.Severity != Warn {
		t.Errorf("Run() with wtype missing: Severity = %v, want Warn", r.Severity)
	}
}

func TestDataDirWritableCheck(t *testing.T) {
	dir := t.TempDir()
	writable := dataDirWritableCheck{resolve: func() (paths.Layout, error) {
		return paths.Layout{Data: filepath.Join(dir, "ingot")}, nil
	}}
	if r := writable.Run(context.Background()); r.Severity != OK {
		t.Errorf("Run() on a writable dir: Severity = %v, want OK", r.Severity)
	}

	unwritable := dataDirWritableCheck{resolve: func() (paths.Layout, error) {
		return paths.Layout{Data: "/proc/does-not-exist-and-cannot-be-created"}, nil
	}}
	if r := unwritable.Run(context.Background()); r.Severity != Warn {
		t.Errorf("Run() on an unwritable dir: Severity = %v, want Warn", r.Severity)
	}
}

func TestSystemdUnitCheck(t *testing.T) {
	notInstalled := systemdUnitCheck{
		unitPaths: func() []string { return []string{filepath.Join(t.TempDir(), "ingot.service")} },
		isEnabled: func(context.Context) (bool, error) {
			t.Fatal("isEnabled must not be called when the unit is absent")
			return false, nil
		},
	}
	if r := notInstalled.Run(context.Background()); r.Severity != OK {
		t.Errorf("Run() with no unit installed: Severity = %v, want OK", r.Severity)
	}

	dir := t.TempDir()
	unitPath := filepath.Join(dir, "ingot.service")
	if err := os.WriteFile(unitPath, []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	enabled := systemdUnitCheck{
		unitPaths: func() []string { return []string{unitPath} },
		isEnabled: func(context.Context) (bool, error) { return true, nil },
	}
	if r := enabled.Run(context.Background()); r.Severity != OK {
		t.Errorf("Run() with unit installed and enabled: Severity = %v, want OK", r.Severity)
	}

	disabled := systemdUnitCheck{
		unitPaths: func() []string { return []string{unitPath} },
		isEnabled: func(context.Context) (bool, error) { return false, nil },
	}
	if r := disabled.Run(context.Background()); r.Severity != Warn {
		t.Errorf("Run() with unit installed but disabled: Severity = %v, want Warn", r.Severity)
	}
	if disabled.Run(context.Background()).Fix == "" {
		t.Error("Run() with unit disabled must set a Fix command")
	}
}

func TestDefaultSystemdUnitPaths(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	candidates := defaultSystemdUnitPaths()
	if len(candidates) == 0 {
		t.Fatal("defaultSystemdUnitPaths() returned no candidates")
	}
	if candidates[0] != "/cfg/systemd/user/ingot.service" {
		t.Errorf("defaultSystemdUnitPaths()[0] = %q, want the XDG_CONFIG_HOME-relative path first", candidates[0])
	}
}
