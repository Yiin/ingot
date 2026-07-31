package setup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Yiin/ingot/internal/store/paths"
	"github.com/Yiin/ingot/internal/wl"
)

var defaultLookPath = exec.LookPath

// waylandSessionCheck fails fatally when no Wayland session is detected:
// Ingot has no X11 or console fallback.
type waylandSessionCheck struct{}

func (waylandSessionCheck) Name() string { return "Wayland session" }

func (waylandSessionCheck) Run(context.Context) Result {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return Result{
			Severity: Fatal,
			Reason:   "WAYLAND_DISPLAY is not set; Ingot is Wayland-only and has no X11 or console fallback",
			Fix:      "log into a Wayland session",
		}
	}
	return ok()
}

// wlProbe abstracts wl.Probe so tests can drive layerShellCheck and
// dataControlCheck without a real Wayland compositor.
type wlProbe func(ctx context.Context) (wl.Capabilities, error)

// layerShellCheck asks the compositor itself, via internal/wl, rather
// than gtk4layershell.IsSupported: internal/setup must stay free of cgo,
// which only internal/ui and internal/layershell may use, and
// zwlr_layer_shell_v1 presence is what gtk4-layer-shell actually depends
// on.
type layerShellCheck struct{ probe wlProbe }

func (layerShellCheck) Name() string { return "layer-shell" }

func (c layerShellCheck) Run(ctx context.Context) Result {
	caps, err := c.probe(ctx)
	if err != nil {
		return Result{Severity: Fatal, Reason: "could not probe the compositor: " + err.Error()}
	}
	if !caps.WlrLayerShell.Present() {
		return Result{
			Severity: Fatal,
			Reason:   "compositor does not advertise zwlr_layer_shell_v1; the panel cannot dock to a screen edge",
			Fix:      "use a compositor that supports wlr-layer-shell (Hyprland, sway, and most other wlroots compositors do)",
		}
	}
	return ok()
}

// readableKeyboardsCheck degrades only the global chord, never the app:
// ingot run still opens the panel from the CLI or a compositor keybind
// with zero keyboards readable.
type readableKeyboardsCheck struct {
	probe func() (KeyboardStatus, error)
}

func (readableKeyboardsCheck) Name() string { return "readable keyboards" }

func (c readableKeyboardsCheck) Run(context.Context) Result {
	status, err := c.probe()
	if err != nil {
		return Result{Severity: Warn, Reason: "could not enumerate input devices: " + err.Error(), Fix: "ingot setup"}
	}
	switch {
	case status.Detected == 0:
		return Result{Severity: Warn, Reason: "no keyboard-capable input devices detected"}
	case status.Readable == 0:
		return Result{
			Severity: Warn,
			Reason:   fmt.Sprintf("detected %d keyboard device(s) but none are readable; the global Shift-Shift chord is disabled", status.Detected),
			Fix:      "ingot setup",
		}
	case status.Readable < status.Detected:
		return Result{
			Severity: Warn,
			Reason:   fmt.Sprintf("%d of %d keyboard devices are readable; some input sources will not trigger the chord", status.Readable, status.Detected),
			Fix:      "ingot setup",
		}
	default:
		return ok()
	}
}

// udevRuleCheck shares installer with `ingot setup` so the two commands
// never disagree about the rule's state.
type udevRuleCheck struct{ installer Installer }

func (udevRuleCheck) Name() string { return "udev rule installed" }

func (c udevRuleCheck) Run(context.Context) Result {
	state, err := c.installer.Status()
	if err != nil {
		return Result{Severity: Warn, Reason: "could not check the udev rule: " + err.Error(), Fix: "ingot setup"}
	}
	switch state {
	case Installed:
		return ok()
	case Modified:
		return Result{Severity: Warn, Reason: RulePath + " exists but its content does not match what Ingot ships", Fix: "ingot setup"}
	default:
		return Result{Severity: Warn, Reason: RulePath + " is not installed", Fix: "ingot setup"}
	}
}

// sessionProbe reads logind session properties. It exists so tests can
// drive activeLocalSessionCheck without a real logind session.
type sessionProbe func(ctx context.Context) (map[string]string, error)

// defaultSessionProbe shells out to loginctl for the caller's own
// session, scoped by XDG_SESSION_ID when set — loginctl's no-argument
// form only resolves a session when invoked from within one, which is
// not guaranteed for every caller.
func defaultSessionProbe(ctx context.Context) (map[string]string, error) {
	args := []string{"show-session"}
	if id := os.Getenv("XDG_SESSION_ID"); id != "" {
		args = append(args, id)
	}
	args = append(args, "-p", "Type", "-p", "Class", "-p", "Active", "-p", "Remote")

	out, err := exec.CommandContext(ctx, "loginctl", args...).Output()
	if err != nil {
		return nil, err
	}

	props := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if k, v, found := strings.Cut(line, "="); found {
			props[k] = v
		}
	}
	return props, nil
}

// activeLocalSessionCheck matters because the udev uaccess ACL this
// package installs only applies inside an active local logind session —
// never over SSH, never from a system service — so a machine can have
// the rule installed and still have unreadable keyboards.
type activeLocalSessionCheck struct{ probe sessionProbe }

func (activeLocalSessionCheck) Name() string { return "active local session" }

func (c activeLocalSessionCheck) Run(ctx context.Context) Result {
	active, reason, err := ActiveLocalSession(ctx, c.probe)
	if err != nil {
		return Result{Severity: Warn, Reason: "could not query the logind session: " + err.Error()}
	}
	if !active {
		return Result{
			Severity: Warn,
			Reason:   reason,
			Fix:      "run Ingot from an active local graphical session, or as a fallback (requires re-login): sudo usermod -aG input $USER",
		}
	}
	return ok()
}

// ActiveLocalSession reports whether the calling process's logind
// session is active and local — the scope the udev uaccess ACL actually
// grants. It is exported so `ingot setup` can warn before installing a
// rule that would not help in the caller's current session.
func ActiveLocalSession(ctx context.Context, probe sessionProbe) (active bool, reason string, err error) {
	if probe == nil {
		probe = defaultSessionProbe
	}
	props, err := probe(ctx)
	if err != nil {
		return false, "", err
	}
	if props["Remote"] == "yes" {
		return false, "this is a remote logind session (e.g. over SSH); the udev uaccess ACL never applies to one", nil
	}
	if props["Active"] != "yes" {
		return false, "this is not the active local logind session; the udev uaccess ACL only applies to the active one", nil
	}
	return true, "", nil
}

// dataControlCheck degrades capture and Copy as List, both of which
// depend on a data-control protocol.
type dataControlCheck struct{ probe wlProbe }

func (dataControlCheck) Name() string { return "data-control" }

func (c dataControlCheck) Run(ctx context.Context) Result {
	caps, err := c.probe(ctx)
	if err != nil {
		return Result{Severity: Warn, Reason: "could not probe the compositor: " + err.Error()}
	}
	if !caps.ExtDataControlManager.Present() && !caps.WlrDataControlManager.Present() {
		return Result{
			Severity: Warn,
			Reason:   "compositor advertises neither ext_data_control_manager_v1 nor zwlr_data_control_manager_v1; capture and Copy as List will both fail",
			Fix:      "use a compositor that supports one of the two data-control protocols (Hyprland, KDE 6.5+, and most other wlroots compositors do)",
		}
	}
	return ok()
}

// clipboardToolsCheck covers wl-copy and wl-paste together: capture and
// Copy as List need both, and they ship in the same package on every
// distribution the epic targets.
type clipboardToolsCheck struct{ lookPath func(string) (string, error) }

func (clipboardToolsCheck) Name() string { return "wl-copy / wl-paste" }

func (c clipboardToolsCheck) Run(context.Context) Result {
	_, copyErr := c.lookPath("wl-copy")
	_, pasteErr := c.lookPath("wl-paste")
	if copyErr == nil && pasteErr == nil {
		return ok()
	}
	return Result{
		Severity: Warn,
		Reason:   "wl-copy and/or wl-paste not found on PATH; capture and Copy as List cannot work",
		Fix:      "install the wl-clipboard package",
	}
}

// wtypeCheck only degrades the opt-in paste-back capture, which the epic
// already treats as a stretch feature.
type wtypeCheck struct{ lookPath func(string) (string, error) }

func (wtypeCheck) Name() string { return "wtype" }

func (c wtypeCheck) Run(context.Context) Result {
	if _, err := c.lookPath("wtype"); err != nil {
		return Result{
			Severity: Warn,
			Reason:   "wtype not found on PATH; the opt-in Ctrl+C paste-back capture is disabled",
			Fix:      "install the wtype package (optional)",
		}
	}
	return ok()
}

var defaultResolveLayout = paths.Resolve

// dataDirWritableCheck actually writes and removes a marker file, since
// a directory can exist and still be unwritable (wrong owner, a
// read-only bind mount, a full disk on some filesystems).
type dataDirWritableCheck struct{ resolve func() (paths.Layout, error) }

func (dataDirWritableCheck) Name() string { return "data dir writable" }

func (c dataDirWritableCheck) Run(context.Context) Result {
	layout, err := c.resolve()
	if err != nil {
		return Result{Severity: Warn, Reason: "could not resolve the data directory: " + err.Error()}
	}
	if err := probeWritable(layout.Data); err != nil {
		return Result{
			Severity: Warn,
			Reason:   fmt.Sprintf("%s is not writable: %v", layout.Data, err),
			Fix:      fmt.Sprintf("check ownership and permissions on %s", layout.Data),
		}
	}
	return ok()
}

func probeWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".ingot-write-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

// systemdUnitName is the unit `ingot doctor --fix` enables and the name
// every unitPaths candidate must end in.
const systemdUnitName = "ingot.service"

// systemdUnitCheck treats an entirely absent unit as healthy: shipping
// and enabling the unit is copper-l2z.32's job, and autostart is opt-in,
// not a requirement for a healthy machine. It only warns once a unit is
// actually installed but not enabled.
type systemdUnitCheck struct {
	unitPaths func() []string
	isEnabled func(ctx context.Context) (bool, error)
}

// SystemdUnitCheckName is systemdUnitCheck's Name(), exported so `ingot
// doctor --fix` can find that check's report and apply FixSystemdUnit only
// when it actually reported something to fix.
const SystemdUnitCheckName = "systemd user unit"

func (systemdUnitCheck) Name() string { return SystemdUnitCheckName }

func (c systemdUnitCheck) Run(ctx context.Context) Result {
	installed := false
	for _, p := range c.unitPaths() {
		if _, err := os.Stat(p); err == nil {
			installed = true
			break
		}
	}
	if !installed {
		return ok()
	}

	enabled, err := c.isEnabled(ctx)
	if err != nil {
		return Result{Severity: Warn, Reason: "could not check whether the systemd user unit is enabled: " + err.Error()}
	}
	if !enabled {
		return Result{
			Severity: Warn,
			Reason:   "the ingot systemd user unit is installed but not enabled, so Ingot will not start automatically",
			Fix:      "ingot doctor --fix (or: systemctl --user enable " + systemdUnitName + ")",
		}
	}
	return ok()
}

func defaultSystemdUnitPaths() []string {
	var out []string
	if cfg := os.Getenv("XDG_CONFIG_HOME"); cfg != "" && filepath.IsAbs(cfg) {
		out = append(out, filepath.Join(cfg, "systemd/user", systemdUnitName))
	} else if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, ".config/systemd/user", systemdUnitName))
	}
	return append(out,
		filepath.Join("/etc/systemd/user", systemdUnitName),
		filepath.Join("/usr/lib/systemd/user", systemdUnitName),
	)
}

func systemdUnitEnabled(ctx context.Context) (bool, error) {
	out, err := exec.CommandContext(ctx, "systemctl", "--user", "is-enabled", systemdUnitName).Output()
	state := strings.TrimSpace(string(out))
	if err != nil {
		// is-enabled exits non-zero for every state that isn't "enabled"
		// (disabled, static, ...); only treat it as a real error when it
		// didn't even report a recognizable state.
		if state != "" {
			return false, nil
		}
		return false, err
	}
	return state == "enabled", nil
}

// FixSystemdUnit applies the one fix `ingot doctor --fix` currently
// knows: enabling an already-installed systemd user unit.
func FixSystemdUnit(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "systemctl", "--user", "enable", systemdUnitName).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("setup: enable %s: %s", systemdUnitName, msg)
		}
		return fmt.Errorf("setup: enable %s: %w", systemdUnitName, err)
	}
	return nil
}
