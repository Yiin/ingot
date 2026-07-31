package setup

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// directRun executes script for real, as the test's own user, ignoring
// bin/args entirely. It stands in for a privileged escalate in tests so
// Install/Uninstall's generated script is proven to actually work — no
// test in this file invokes pkexec, sudo, or anything that needs root.
func directRun(bin string, args []string, script, stdin string) error {
	cmd := exec.Command("sh", "-c", script)
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

// alwaysFound is a lookPath stub that pretends every binary exists,
// returning its own name as the resolved path.
func alwaysFound(name string) (string, error) { return name, nil }

func writeFakeBinary(t *testing.T, dir, name, script string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
}

// TestRunEscalated exercises the real production escalate function —
// every other test in this file substitutes directRun, which reimplements
// the same `sh -c` plumbing and would not catch a bug specific to
// runEscalated's own arg assembly or stdin wiring. /usr/bin/env stands in
// for pkexec/sudo here since `env sh -c script` runs script unprivileged,
// exactly like `pkexec sh -c script` would run it as root.
func TestRunEscalated(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")

	if err := runEscalated("/usr/bin/env", nil, "cat > "+marker, "hello from stdin"); err != nil {
		t.Fatalf("runEscalated() error = %v", err)
	}
	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if string(got) != "hello from stdin" {
		t.Errorf("marker content = %q, want %q", got, "hello from stdin")
	}

	if err := runEscalated("/usr/bin/env", nil, "exit 1", ""); err == nil {
		t.Error("runEscalated() with a failing script: error = nil, want an error")
	}
}

func TestRealInstaller_Status(t *testing.T) {
	dir := t.TempDir()
	rulePath := filepath.Join(dir, "70-ingot-input.rules")

	in := newInstaller(rulePath, dir, os.ReadFile, alwaysFound, directRun)

	state, err := in.Status()
	if err != nil {
		t.Fatalf("Status() on missing file: err = %v", err)
	}
	if state != NotInstalled {
		t.Errorf("Status() on missing file = %v, want NotInstalled", state)
	}

	if err := os.WriteFile(rulePath, []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err = in.Status()
	if err != nil {
		t.Fatalf("Status() on mismatched content: err = %v", err)
	}
	if state != Modified {
		t.Errorf("Status() on mismatched content = %v, want Modified", state)
	}

	if err := os.WriteFile(rulePath, []byte(RuleContent), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err = in.Status()
	if err != nil {
		t.Fatalf("Status() on matching content: err = %v", err)
	}
	if state != Installed {
		t.Errorf("Status() on matching content = %v, want Installed", state)
	}
}

func TestRealInstaller_Status_ReadError(t *testing.T) {
	readErr := errors.New("boom")
	in := newInstaller("/does/not/matter", "/does/not/matter", func(string) ([]byte, error) {
		return nil, readErr
	}, alwaysFound, directRun)

	if _, err := in.Status(); !errors.Is(err, readErr) {
		t.Errorf("Status() error = %v, want it to wrap %v", err, readErr)
	}
}

// TestRealInstaller_InstallEndToEnd runs Install for real against a temp
// directory and a fake udevadm on PATH, proving the generated script
// actually writes RuleContent byte for byte and invokes both reload
// commands in order — the mechanics the "makes keyboards readable after
// udevadm trigger" acceptance criterion depends on. It needs no root: the
// privileged escalation step is stubbed by directRun.
func TestRealInstaller_InstallEndToEnd(t *testing.T) {
	work := t.TempDir()
	ruleDir := filepath.Join(work, "rules.d")
	rulePath := filepath.Join(ruleDir, RuleName)

	binDir := t.TempDir()
	log := filepath.Join(work, "udevadm.log")
	writeFakeBinary(t, binDir, "udevadm", "#!/bin/sh\necho \"udevadm $*\" >> \""+log+"\"\n")
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	in := newInstaller(rulePath, ruleDir, os.ReadFile, alwaysFound, directRun)

	if state, err := in.Status(); err != nil || state != NotInstalled {
		t.Fatalf("Status() before install = (%v, %v), want (NotInstalled, nil)", state, err)
	}

	if err := in.Install(); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	data, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("read installed rule: %v", err)
	}
	if string(data) != RuleContent {
		t.Errorf("installed content = %q, want %q", data, RuleContent)
	}

	if state, err := in.Status(); err != nil || state != Installed {
		t.Fatalf("Status() after install = (%v, %v), want (Installed, nil)", state, err)
	}

	logData, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("read udevadm log: %v", err)
	}
	got := string(logData)
	if !strings.Contains(got, "udevadm control --reload-rules") || !strings.Contains(got, "udevadm trigger") {
		t.Errorf("udevadm log = %q, want both reload commands", got)
	}
	// The reload must come after control --reload-rules, matching
	// ReloadCommands' order.
	if strings.Index(got, "control") > strings.Index(got, "trigger") {
		t.Errorf("udevadm log = %q, want reload-rules before trigger", got)
	}
}

func TestRealInstaller_UninstallEndToEnd(t *testing.T) {
	work := t.TempDir()
	ruleDir := filepath.Join(work, "rules.d")
	rulePath := filepath.Join(ruleDir, RuleName)
	if err := os.MkdirAll(ruleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rulePath, []byte(RuleContent), 0o644); err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	writeFakeBinary(t, binDir, "udevadm", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	in := newInstaller(rulePath, ruleDir, os.ReadFile, alwaysFound, directRun)

	if err := in.Uninstall(); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	if _, err := os.Stat(rulePath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("rule file still exists after Uninstall(): err = %v", err)
	}
	if state, err := in.Status(); err != nil || state != NotInstalled {
		t.Fatalf("Status() after uninstall = (%v, %v), want (NotInstalled, nil)", state, err)
	}
}

func TestRealInstaller_EscalationCommand(t *testing.T) {
	tests := []struct {
		name      string
		lookPath  func(string) (string, error)
		wantBin   string
		wantError bool
	}{
		{
			name: "prefers pkexec",
			lookPath: func(bin string) (string, error) {
				if bin == "pkexec" || bin == "sudo" {
					return "/usr/bin/" + bin, nil
				}
				return "", exec.ErrNotFound
			},
			wantBin: "/usr/bin/pkexec",
		},
		{
			name: "falls back to sudo",
			lookPath: func(bin string) (string, error) {
				if bin == "sudo" {
					return "/usr/bin/sudo", nil
				}
				return "", exec.ErrNotFound
			},
			wantBin: "/usr/bin/sudo",
		},
		{
			name:      "errors when neither is present",
			lookPath:  func(string) (string, error) { return "", exec.ErrNotFound },
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := newInstaller("rule", "dir", os.ReadFile, tt.lookPath, directRun)
			bin, _, err := in.escalationCommand()
			if tt.wantError {
				if err == nil {
					t.Fatal("escalationCommand() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("escalationCommand() error = %v", err)
			}
			if bin != tt.wantBin {
				t.Errorf("escalationCommand() bin = %q, want %q", bin, tt.wantBin)
			}
		})
	}
}

// TestInstallScript_MatchesDisplay guards against the exact bug the
// review caught: cli.go must show the user the same script Install()
// actually runs. Since both derive from installScript, this only checks
// that the exported display helpers still route through it.
func TestInstallScript_MatchesDisplay(t *testing.T) {
	if got, want := InstallScript(), installScript(RuleDir, RulePath); got != want {
		t.Errorf("InstallScript() = %q, want %q", got, want)
	}
	if got, want := UninstallScript(), uninstallScript(RulePath); got != want {
		t.Errorf("UninstallScript() = %q, want %q", got, want)
	}
	for _, c := range reloadCommands {
		if !strings.Contains(InstallScript(), c) {
			t.Errorf("InstallScript() = %q, want it to contain %q", InstallScript(), c)
		}
	}
}

func TestFakeInstaller(t *testing.T) {
	f := NewFakeInstaller(NotInstalled)

	if state, err := f.Status(); err != nil || state != NotInstalled {
		t.Fatalf("Status() = (%v, %v), want (NotInstalled, nil)", state, err)
	}
	if err := f.Install(); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if state, _ := f.Status(); state != Installed {
		t.Errorf("Status() after Install() = %v, want Installed", state)
	}
	if err := f.Uninstall(); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if state, _ := f.Status(); state != NotInstalled {
		t.Errorf("Status() after Uninstall() = %v, want NotInstalled", state)
	}

	wantErr := errors.New("denied")
	f.InstallErr = wantErr
	if err := f.Install(); !errors.Is(err, wantErr) {
		t.Errorf("Install() error = %v, want %v", err, wantErr)
	}
}
