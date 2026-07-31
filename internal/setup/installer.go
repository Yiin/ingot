package setup

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// State is the udev rule's on-disk state relative to what Ingot ships.
type State int

const (
	// NotInstalled means RulePath does not exist.
	NotInstalled State = iota
	// Installed means RulePath exists and matches RuleContent byte for
	// byte.
	Installed
	// Modified means RulePath exists but its content differs from
	// RuleContent — most likely an older version of the rule, or a hand
	// edit.
	Modified
)

func (s State) String() string {
	switch s {
	case Installed:
		return "installed"
	case Modified:
		return "modified"
	default:
		return "not installed"
	}
}

// Installer manages the udev rule's installed state.
type Installer interface {
	Status() (State, error)
	Install() error
	Uninstall() error
}

// escalateFunc runs script as root via bin (pkexec or sudo), piping stdin
// to it when non-empty, and returns any stderr output as the error text.
// It exists so tests can drive Install/Uninstall's exact command
// composition without a real privileged process.
type escalateFunc func(bin string, args []string, script, stdin string) error

// RealInstaller manages the udev rule on the real filesystem, escalating
// to root via pkexec or sudo to write or remove it.
type RealInstaller struct {
	rulePath string
	ruleDir  string

	readFile func(string) ([]byte, error)
	lookPath func(string) (string, error)
	escalate escalateFunc
}

var _ Installer = (*RealInstaller)(nil)

// NewInstaller returns an Installer backed by the real RulePath, PATH,
// and a real pkexec/sudo invocation.
func NewInstaller() *RealInstaller {
	return newInstaller(RulePath, RuleDir, os.ReadFile, exec.LookPath, runEscalated)
}

func newInstaller(rulePath, ruleDir string, readFile func(string) ([]byte, error), lookPath func(string) (string, error), escalate escalateFunc) *RealInstaller {
	return &RealInstaller{
		rulePath: rulePath,
		ruleDir:  ruleDir,
		readFile: readFile,
		lookPath: lookPath,
		escalate: escalate,
	}
}

// Status reads RulePath and compares it against RuleContent byte for
// byte. Reading never needs privilege: the rule file is world-readable
// once installed.
func (in *RealInstaller) Status() (State, error) {
	data, err := in.readFile(in.rulePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NotInstalled, nil
		}
		return NotInstalled, fmt.Errorf("setup: read %s: %w", in.rulePath, err)
	}
	if string(data) == RuleContent {
		return Installed, nil
	}
	return Modified, nil
}

// Install writes RuleContent to rulePath and reloads udev, all through a
// single privilege escalation.
func (in *RealInstaller) Install() error {
	bin, args, err := in.escalationCommand()
	if err != nil {
		return err
	}
	if err := in.escalate(bin, args, installScript(in.ruleDir, in.rulePath), RuleContent); err != nil {
		return fmt.Errorf("setup: install %s: %w", in.rulePath, err)
	}
	return nil
}

// Uninstall removes rulePath and reloads udev, all through a single
// privilege escalation.
func (in *RealInstaller) Uninstall() error {
	bin, args, err := in.escalationCommand()
	if err != nil {
		return err
	}
	if err := in.escalate(bin, args, uninstallScript(in.rulePath), ""); err != nil {
		return fmt.Errorf("setup: uninstall %s: %w", in.rulePath, err)
	}
	return nil
}

// installScript and uninstallScript build the exact `sh -c` script Install
// and Uninstall run as root. They are the single source of truth for that
// script: cli.go's InstallScript/UninstallScript helpers call them too, so
// the command a user is shown before authorizing can never drift from the
// command that actually runs.
func installScript(ruleDir, rulePath string) string {
	return fmt.Sprintf(
		"mkdir -p %s && tee %s >/dev/null && chmod 0644 %s && %s",
		shQuote(ruleDir), shQuote(rulePath), shQuote(rulePath), strings.Join(reloadCommands, " && "),
	)
}

func uninstallScript(rulePath string) string {
	return fmt.Sprintf("rm -f %s && %s", shQuote(rulePath), strings.Join(reloadCommands, " && "))
}

// InstallScript returns the exact shell script `ingot setup` runs as root
// to install the rule, for display to the user before they authorize it.
func InstallScript() string { return installScript(RuleDir, RulePath) }

// UninstallScript returns the exact shell script `ingot setup --uninstall`
// runs as root to remove the rule, for display to the user before they
// authorize it.
func UninstallScript() string { return uninstallScript(RulePath) }

// escalationCommand picks pkexec over sudo when both are present: pkexec
// prompts through the desktop's own polkit agent, which fits a
// keyboard-first GUI app better than a terminal sudo password prompt.
func (in *RealInstaller) escalationCommand() (string, []string, error) {
	if p, err := in.lookPath("pkexec"); err == nil {
		return p, nil, nil
	}
	if p, err := in.lookPath("sudo"); err == nil {
		return p, nil, nil
	}
	return "", nil, errors.New("setup: neither pkexec nor sudo is available; install the rule manually")
}

// runEscalated runs `bin args... sh -c script`, feeding stdin to it when
// non-empty, and folds stderr into the returned error so a failure names
// the actual command that failed rather than just its exit status.
func runEscalated(bin string, args []string, script, stdin string) error {
	full := append(append([]string{}, args...), "sh", "-c", script)
	cmd := exec.Command(bin, full...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return errors.New(msg)
		}
		return err
	}
	return nil
}

// shQuote wraps s in single quotes for embedding in a `sh -c` script,
// escaping any single quote it contains. Every path this package quotes
// is a compile-time constant, but quoting defensively costs nothing and
// means a future caller can't reintroduce a shell-injection bug by
// pointing rulePath/ruleDir at a caller-supplied value.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// FakeInstaller is an in-memory Installer for tests: it never touches the
// filesystem or invokes a privileged process.
type FakeInstaller struct {
	state State

	// InstallErr and UninstallErr let a test force a failure path.
	InstallErr   error
	UninstallErr error
}

var _ Installer = (*FakeInstaller)(nil)

// NewFakeInstaller returns a FakeInstaller starting in the given state.
func NewFakeInstaller(initial State) *FakeInstaller {
	return &FakeInstaller{state: initial}
}

func (f *FakeInstaller) Status() (State, error) { return f.state, nil }

func (f *FakeInstaller) Install() error {
	if f.InstallErr != nil {
		return f.InstallErr
	}
	f.state = Installed
	return nil
}

func (f *FakeInstaller) Uninstall() error {
	if f.UninstallErr != nil {
		return f.UninstallErr
	}
	f.state = NotInstalled
	return nil
}
