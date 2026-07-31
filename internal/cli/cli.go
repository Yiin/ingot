// Package cli dispatches Ingot's subcommands.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yiin/ingot/internal/app"
	"github.com/Yiin/ingot/internal/buildinfo"
	"github.com/Yiin/ingot/internal/setup"
	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/mdfile"
	"github.com/Yiin/ingot/internal/store/paths"
)

const usage = `Ingot — a keyboard-first quick-capture panel for Wayland.

Usage:
  ingot                  Toggle the panel (launches it if not already running)
  ingot run [--hidden]     Launch the panel
  ingot doctor          Diagnose permissions and compositor support
  ingot setup            Install the udev rule and other first-run setup
  ingot version          Print the version
  ingot path [kind]       Print an XDG path (data, projects, meta, backups, trash, config, state), or a project's file
  ingot export <project>    Export a project's Markdown to stdout
  ingot import <file.md|->  Import a Markdown file as a new project
`

// Run dispatches args (as passed to main, including the program name at
// index 0) to the matching subcommand and exits the process on failure.
// With no subcommand at all, it dispatches to "run": a bare `ingot`
// toggles the panel of an already-running instance, or becomes the
// primary instance if there isn't one — see runRun.
func Run(args []string) {
	sub := "run"
	var rest []string
	if len(args) >= 2 {
		sub = args[1]
		rest = args[2:]
	}

	var err error
	switch sub {
	case "run":
		err = runRun(rest)
	case "doctor":
		err = runDoctor(rest)
	case "setup":
		err = runSetup(rest)
	case "version":
		fmt.Println(buildinfo.Version())
	case "path":
		err = runPath(rest)
	case "export":
		err = runExport(rest)
	case "import":
		err = runImport(rest)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "ingot:", err)
		os.Exit(1)
	}
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

func runRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	hidden := fs.Bool("hidden", false, "start with the panel hidden — for a systemd user unit that shouldn't steal focus at login")
	if err := fs.Parse(args); err != nil {
		return err
	}
	os.Exit(app.Run(app.Options{Hidden: *hidden}))
	return nil // unreachable
}

// pathKinds are the Layout fields "ingot path <kind>" can print directly,
// in the order a bare "ingot path" lists all of them.
var pathKinds = []string{"data", "projects", "meta", "backups", "trash", "config", "state"}

func pathForKind(layout paths.Layout, kind string) (string, bool) {
	switch kind {
	case "data":
		return layout.Data, true
	case "projects":
		return layout.Projects, true
	case "meta":
		return layout.Meta, true
	case "backups":
		return layout.Backups, true
	case "trash":
		return layout.Trash, true
	case "config":
		return layout.Config, true
	case "state":
		return layout.State, true
	default:
		return "", false
	}
}

// runPath prints an XDG directory (ingot path <data|projects|meta|
// backups|trash|config|state>, or every one of them with no argument),
// or — for anything else — the Markdown file of the project whose title
// or slug matches, so "ingot path <project>" (the child spec's own
// wording) and "ingot path <kind>" (the shipped usage string's wording)
// both work.
func runPath(args []string) error {
	layout, err := paths.Resolve()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		for _, k := range pathKinds {
			p, _ := pathForKind(layout, k)
			fmt.Printf("%s\t%s\n", k, p)
		}
		return nil
	}

	arg := args[0]
	if p, ok := pathForKind(layout, arg); ok {
		fmt.Println(p)
		return nil
	}

	file, err := resolveProjectFile(layout, arg)
	if err != nil {
		return fmt.Errorf("path: %w", err)
	}
	fmt.Println(file)
	return nil
}

// resolveProjectFile finds the Markdown file for a project named by
// title or slug. fsstore is never constructed for this — New's own
// pruneTrash side effect would be a surprise from a read-only CLI
// command — so this reads the directory directly.
func resolveProjectFile(layout paths.Layout, name string) (string, error) {
	file, err := paths.ProjectFile(layout, paths.Slug(name))
	if err != nil {
		return "", err
	}
	if _, err := fsx.OS().Stat(file); err != nil {
		return "", fmt.Errorf("no project matches %q (looked for %s)", name, file)
	}
	return file, nil
}

// runExport prints a project's Markdown, either byte-for-byte (--raw) or
// re-formatted through mdfile.Parse/Format — the same round trip every
// project file already goes through on its own next save, so this is
// never lossier than what Ingot would have written anyway.
func runExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	raw := fs.Bool("raw", false, "copy the file's bytes verbatim instead of re-formatting through mdfile")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("export: usage: ingot export [--raw] <project>")
	}

	layout, err := paths.Resolve()
	if err != nil {
		return err
	}
	file, err := resolveProjectFile(layout, fs.Arg(0))
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	data, err := fsx.OS().ReadFile(file)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	if *raw {
		_, err := os.Stdout.Write(data)
		return err
	}

	proj, warnings, err := mdfile.Parse(data)
	if err != nil {
		return fmt.Errorf("export: parse %s: %w", file, err)
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "ingot: export:", w)
	}
	out, err := mdfile.Format(proj)
	if err != nil {
		return fmt.Errorf("export: format: %w", err)
	}
	_, err = os.Stdout.Write(out)
	return err
}

// runImport reads a Markdown project (a file path, or "-" for stdin) and
// writes it under Layout.Projects, minting a fresh id for a file that
// never had one. A project whose front-matter id already names an
// existing project refuses to overwrite it without --force, mirroring
// how fsstore treats an id collision at load — made explicit here
// instead of silently minting a second id.
func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	force := fs.Bool("force", false, "overwrite an existing project that has the same id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("import: usage: ingot import [--force] <file.md|->")
	}

	src := fs.Arg(0)
	var data []byte
	var err error
	if src == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(src)
	}
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	proj, warnings, err := mdfile.Parse(data)
	if err != nil {
		return fmt.Errorf("import: parse: %w", err)
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "ingot: import:", w)
	}

	layout, err := paths.Resolve()
	if err != nil {
		return err
	}
	if err := fsx.OS().MkdirAll(layout.Projects, 0o755); err != nil {
		return fmt.Errorf("import: %w", err)
	}

	slugs, byID, err := existingProjects(layout)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	slug := paths.UniqueSlug(slugs, paths.Slug(firstNonEmpty(proj.Title, "Imported")))
	if proj.ID != "" {
		if existingFile, ok := byID[string(proj.ID)]; ok {
			if !*force {
				return fmt.Errorf("import: a project with id %q already exists at %s; pass --force to overwrite", proj.ID, existingFile)
			}
			slug = strings.TrimSuffix(filepath.Base(existingFile), ".md")
		}
	} else {
		proj.ID = store.ProjectID(store.NewID())
	}
	if proj.Created.IsZero() {
		proj.Created = time.Now()
	}

	out, err := mdfile.Format(proj)
	if err != nil {
		return fmt.Errorf("import: format: %w", err)
	}

	file, err := paths.ProjectFile(layout, slug)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}
	if err := fsx.AtomicWrite(fsx.OS(), file, out); err != nil {
		return fmt.Errorf("import: write: %w", err)
	}

	fmt.Println("Imported:", file)
	return nil
}

// existingProjects scans Layout.Projects for every project's slug and,
// where its front matter carries one, its id — used by import to check
// for an id collision and mint a slug that doesn't clash with an
// existing file. A missing Projects directory is not an error: there
// simply are no existing projects yet.
func existingProjects(layout paths.Layout) (slugs []string, byID map[string]string, err error) {
	byID = make(map[string]string)

	entries, err := fsx.OS().ReadDir(layout.Projects)
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return nil, byID, nil
		}
		return nil, nil, err
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		slugs = append(slugs, strings.TrimSuffix(e.Name(), ".md"))

		file := filepath.Join(layout.Projects, e.Name())
		data, err := fsx.OS().ReadFile(file)
		if err != nil {
			continue
		}
		p, _, err := mdfile.Parse(data)
		if err != nil || p.ID == "" {
			continue
		}
		byID[string(p.ID)] = file
	}
	return slugs, byID, nil
}

func firstNonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
