// Package cli dispatches Ingot's subcommands.
package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Yiin/ingot/internal/buildinfo"
	"github.com/Yiin/ingot/internal/setup"
)

const usage = `Ingot — a keyboard-first quick-capture panel for Wayland.

Usage:
  ingot run            Launch the panel
  ingot doctor          Diagnose permissions and compositor support
  ingot setup            Install the udev rule and other first-run setup
  ingot version          Print the version
  ingot path <kind>       Print an XDG path (data, config, state)
  ingot export <target>    Export notes
  ingot import <source>    Import notes
`

// Run dispatches args (as passed to main, including the program name at
// index 0) to the matching subcommand and exits the process on failure.
func Run(args []string) {
	if len(args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch args[1] {
	case "run":
		err = runRun(args[2:])
	case "doctor":
		err = runDoctor(args[2:])
	case "setup":
		err = runSetup(args[2:])
	case "version":
		fmt.Println(buildinfo.Version())
	case "path":
		err = runPath(args[2:])
	case "export":
		err = runExport(args[2:])
	case "import":
		err = runImport(args[2:])
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "ingot:", err)
		os.Exit(1)
	}
}

func runRun(args []string) error {
	return fmt.Errorf("run: not implemented yet")
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fix := fs.Bool("fix", false, "apply automatic fixes where possible (currently: enable the systemd user unit)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	reports := setup.RunChecks(ctx, setup.DefaultChecks(setup.NewInstaller()))

	for _, r := range reports {
		fmt.Printf("[%s] %s\n", strings.ToUpper(r.Result.Severity.String()), r.Name)
		if r.Result.Reason != "" {
			fmt.Println("  " + r.Result.Reason)
		}
		if r.Result.Fix != "" {
			fmt.Println("  fix: " + r.Result.Fix)
		}
	}

	if *fix {
		for _, r := range reports {
			if r.Name != setup.SystemdUnitCheckName || r.Result.Severity == setup.OK {
				continue
			}
			if err := setup.FixSystemdUnit(ctx); err != nil {
				fmt.Fprintln(os.Stderr, "ingot: doctor --fix:", err)
			} else {
				fmt.Println("Fixed: systemd user unit enabled. Run `ingot doctor` again to confirm.")
			}
		}
	}

	if !setup.Healthy(reports) {
		return fmt.Errorf("doctor: one or more checks need attention; see fix commands above")
	}
	return nil
}

func runSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	check := fs.Bool("check", false, "check whether the udev rule is installed and exit non-zero if not, without changing anything")
	uninstall := fs.Bool("uninstall", false, "remove the udev rule")
	if err := fs.Parse(args); err != nil {
		return err
	}

	installer := setup.NewInstaller()

	if *check {
		state, err := installer.Status()
		if err != nil {
			return err
		}
		if state != setup.Installed {
			return fmt.Errorf("%s is %s; run `ingot setup` to install it", setup.RulePath, state)
		}
		fmt.Println("udev rule installed:", setup.RulePath)
		return nil
	}

	if *uninstall {
		fmt.Println("Removing the udev rule and reloading udev, run once via pkexec or sudo:")
		fmt.Println("  " + setup.UninstallScript())
		if err := installer.Uninstall(); err != nil {
			return fmt.Errorf("setup: uninstall: %w", err)
		}
		fmt.Println("Removed", setup.RulePath)
		return nil
	}

	state, err := installer.Status()
	if err != nil {
		return err
	}
	if state == setup.Installed {
		fmt.Println("udev rule already installed:", setup.RulePath)
		return nil
	}

	ctx := context.Background()
	if active, reason, err := setup.ActiveLocalSession(ctx, nil); err == nil && !active {
		fmt.Println("Warning: " + reason + ".")
		fmt.Println("The rule below only grants access inside an active local session, so it will not help here.")
		fmt.Println("Fallback, which requires a re-login to take effect: sudo usermod -aG input $USER")
		fmt.Println()
	}
	if state == setup.Modified {
		fmt.Println("Note: " + setup.RulePath + " already exists with different content; this will replace it.")
		fmt.Println()
	}

	fmt.Println("Ingot needs to read keyboard input to detect the global Shift-Shift chord.")
	fmt.Println("Rather than adding your user to the input group — permanent, and covering every input")
	fmt.Println("device including mice — Ingot installs a udev rule scoped to just keyboards, active only")
	fmt.Println("inside your current local session:")
	fmt.Println()
	fmt.Println("  " + setup.RulePath)
	for _, line := range strings.Split(strings.TrimRight(setup.RuleContent, "\n"), "\n") {
		fmt.Println("    " + line)
	}
	fmt.Println()
	fmt.Println("This requires the following command, run once via pkexec or sudo:")
	fmt.Println("  " + setup.InstallScript())
	fmt.Println()

	if err := installer.Install(); err != nil {
		return fmt.Errorf("setup: install: %w", err)
	}

	state, err = installer.Status()
	if err != nil {
		return err
	}
	if state != setup.Installed {
		return fmt.Errorf("setup: installed %s but it does not verify byte-for-byte afterward; check it manually", setup.RulePath)
	}
	fmt.Println("Installed and reloaded. Keyboards should be readable now — run `ingot doctor` to confirm.")
	return nil
}

func runPath(args []string) error {
	return fmt.Errorf("path: not implemented yet")
}

func runExport(args []string) error {
	return fmt.Errorf("export: not implemented yet")
}

func runImport(args []string) error {
	return fmt.Errorf("import: not implemented yet")
}
