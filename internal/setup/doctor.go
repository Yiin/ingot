package setup

import (
	"context"

	"github.com/Yiin/ingot/internal/wl"
)

// Severity classifies how much a failed Check matters. Only a check that
// blocks a core feature entirely is Fatal; everything else degrades one
// optional capability and is Warn — the same "a missing permission must
// degrade one feature, not the app" rule the udev rule exists to serve.
type Severity int

const (
	// OK means the check passed; Reason and Fix are empty.
	OK Severity = iota
	// Warn means one feature is degraded; Reason explains which, and Fix
	// names the exact command that resolves it, when one exists.
	Warn
	// Fatal means a core capability (the Wayland session itself, or the
	// panel's ability to dock via layer-shell) is unavailable.
	Fatal
)

func (s Severity) String() string {
	switch s {
	case Warn:
		return "warn"
	case Fatal:
		return "fatal"
	default:
		return "ok"
	}
}

// Result is one Check's outcome.
type Result struct {
	Severity Severity
	// Reason is a one-line explanation. It is set whenever Severity is
	// not OK, and never a bare "permission denied" — every failure names
	// what it means for Ingot.
	Reason string
	// Fix is the exact command that resolves Reason, when one exists.
	Fix string
}

func ok() Result { return Result{Severity: OK} }

// Check is one first-run or ongoing health check `ingot doctor` runs.
type Check interface {
	Name() string
	Run(ctx context.Context) Result
}

// Report pairs a Check's name with the Result of running it.
type Report struct {
	Name   string
	Result Result
}

// RunChecks runs every check in order and collects its Report. Checks
// are independent and cheap (no check here talks to more than one
// process), so they run sequentially in the order given — the order
// `ingot doctor` prints them in.
func RunChecks(ctx context.Context, checks []Check) []Report {
	reports := make([]Report, len(checks))
	for i, c := range checks {
		reports[i] = Report{Name: c.Name(), Result: c.Run(ctx)}
	}
	return reports
}

// Healthy reports whether every report's Severity is OK.
func Healthy(reports []Report) bool {
	for _, r := range reports {
		if r.Result.Severity != OK {
			return false
		}
	}
	return true
}

// DefaultChecks returns every check `ingot doctor` runs against the real
// machine, reusing installer for the udev-rule check so `ingot setup`
// and `ingot doctor` never disagree about the rule's state.
func DefaultChecks(installer Installer) []Check {
	return []Check{
		waylandSessionCheck{},
		layerShellCheck{probe: wl.Probe},
		readableKeyboardsCheck{probe: func() (KeyboardStatus, error) { return ProbeKeyboards("", "") }},
		udevRuleCheck{installer: installer},
		activeLocalSessionCheck{probe: defaultSessionProbe},
		dataControlCheck{probe: wl.Probe},
		clipboardToolsCheck{lookPath: defaultLookPath},
		wtypeCheck{lookPath: defaultLookPath},
		dataDirWritableCheck{resolve: defaultResolveLayout},
		systemdUnitCheck{unitPaths: defaultSystemdUnitPaths, isEnabled: systemdUnitEnabled},
	}
}
