// Package cli dispatches Ingot's subcommands.
package cli

import (
	"fmt"
	"os"

	"github.com/Yiin/ingot/internal/buildinfo"
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
	return fmt.Errorf("doctor: not implemented yet")
}

func runSetup(args []string) error {
	return fmt.Errorf("setup: not implemented yet")
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
