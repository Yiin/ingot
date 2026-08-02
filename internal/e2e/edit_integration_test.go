//go:build integration

// TestInlineEdit_RewritesTheNoteInPlace is the regression test for the
// inline editor being built and never called.
//
// notelist.StartInlineEdit, widget.Row.StartEdit and the whole of
// internal/ui/editorwindow were implemented and unit tested while
// App.Edit, App.EditNewWindow and App.Expand were empty method bodies —
// so Return, Ctrl+Return, Right and Left were listed in the shortcuts
// window (Ctrl+?) and did nothing at all. Every package's own tests
// passed throughout, exactly as they did for the Escape cascade that
// TestEscape_HidesThePanel exists to guard; the same reasoning applies
// here, so this lives beside it rather than in internal/app.
//
// Unit tests cannot close this. notelist's tests drive StartInlineEdit
// directly and pass whether or not any key reaches it, and internal/app's
// own tests can only assert that a handler is present in a map. Only
// pressing the key against the built binary proves the path from keyboard
// to store is unbroken.
//
// The assertion is deliberately shaped to tell an inline edit apart from
// the failure it would otherwise be indistinguishable from. If the key
// landed anywhere else the typed marker would go to the main composer,
// and Return would commit it as a *second* note — so a passing run
// requires both that the marker is on disk and that the project still
// holds exactly one note.
package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// tabsFromComposerToList is how many Tab presses move focus off the
// composer and onto a note row. Measured by logging the window's focus
// widget for every key under this same harness: Tab 1 lands on the search
// entry, Tab 2 on the overflow button, Tab 3 on a GtkListItemWidget.
const tabsFromComposerToList = 3

func TestInlineEdit_RewritesTheNoteInPlace(t *testing.T) {
	requireHeadlessSession(t)
	requireTool(t, "grim")
	requireTool(t, "wtype")

	bin := buildIngot(t)
	env, dataHome := isolatedXDGEnv(t)
	projects := filepath.Join(dataHome, "ingot", "projects")

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, "run")
	cmd.Env = env
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting %s run: %v", bin, err)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-exited:
			case <-time.After(3 * time.Second):
				_ = cmd.Process.Kill()
			}
		}
	})

	select {
	case err := <-exited:
		t.Fatalf("ingot run exited early — err=%v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	case <-time.After(4 * time.Second):
	}

	// An empty store puts the panel in its hint state, which is the one
	// case where it focuses the composer itself — so the starting focus
	// here is known rather than whatever GTK picked.
	//
	// The throwaway leading key is copper-l2z.83's finding: a fresh wtype
	// process drops its own first key event.
	wtype(t, "-k", "End", "-s", "250", "alpha", "-s", "250", "-k", "Return")

	if !pollForContent(t, projects, "alpha", 5*time.Second) {
		t.Fatalf("note never reached disk; composer wiring is broken before this test's subject\nstderr:\n%s", stderr.String())
	}

	// Reach the list, open the row's inline editor, append a marker,
	// commit. Nothing mutates the note first: doing so leaves the row
	// unable to take focus into its editor at all (copper-c0u), which is
	// a separate defect this test must not be entangled with.
	//
	// No Down among the Tabs on purpose. Tab alone must be enough: it
	// focuses a row without selecting it and without telling keymap.Nav,
	// which is exactly the state that used to make every list command a
	// silent no-op.
	args := []string{"-k", "End"}
	for i := 0; i < tabsFromComposerToList; i++ {
		args = append(args, "-s", "250", "-k", "Tab")
	}
	args = append(args,
		"-s", "400", "-k", "Return", // edit-inline, through keymap.InstallListGate
		"-s", "600", "-k", "End", // caret to the end of the seeded body
		"-s", "250", "ZULU",
		"-s", "250", "-k", "Return", // commit
	)
	wtype(t, args...)

	if !pollForContent(t, projects, "ZULU", 5*time.Second) {
		t.Fatalf("inline edit never reached the store — Return on a focused row did not start or commit an edit "+
			"(regression: App.Edit is not wired into keymap.InstallListGate)\nproject file:\n%s\nstderr:\n%s",
			readProjects(t, projects), stderr.String())
	}

	// The discriminator: an edit rewrites the note it was opened on. A
	// second note means the keystrokes went to the main composer instead
	// and this test proved nothing about inline editing.
	if n := countNotes(t, projects); n != 1 {
		t.Fatalf("expected the one note to have been rewritten in place, found %d notes — "+
			"the marker was typed into the composer, not an inline editor\nproject file:\n%s",
			n, readProjects(t, projects))
	}
}

// wtype runs one wtype invocation and fails the test if it errors.
func wtype(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command("wtype", args...).CombinedOutput(); err != nil {
		t.Fatalf("wtype %v: %v: %s", args, err, out)
	}
}

// countNotes counts note lines across every project Markdown file in dir.
// mdfile writes one note per "- [ ] " / "- [x] " line, so counting those
// is counting notes.
func countNotes(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	for _, line := range strings.Split(readProjects(t, dir), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ] ") || strings.HasPrefix(trimmed, "- [x] ") {
			n++
		}
	}
	return n
}

// readProjects concatenates every project Markdown file in dir, for
// counting and for failure messages.
func readProjects(t *testing.T, dir string) string {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	var b strings.Builder
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		b.WriteString(string(data))
		b.WriteString("\n")
	}
	return b.String()
}

// TestListKeys_ActOnATabFocusedRow is the regression test for list
// commands doing nothing after the list is reached by Tab.
//
// keymap.Nav tracks its own focused row, but it only learns about moves
// its own key controller drove. Tab is GTK's, not Nav's, so tabbing to a
// row left Nav with no focus and the GtkMultiSelection with no selection
// — and every list command reads one or the other. Measured before the
// fix: Tab, Tab, Tab, Space reached the gate with the row focused
// ("GtkListItemWidget [activatable]") and marked nothing done.
//
// This is a separate test from the inline-edit one, not a step inside it,
// because marking a note done makes that same row unable to focus its own
// inline editor (copper-c0u). Separate tests get separate stores and
// separate processes, so neither can set the other up to fail.
func TestListKeys_ActOnATabFocusedRow(t *testing.T) {
	requireHeadlessSession(t)
	requireTool(t, "wtype")

	bin := buildIngot(t)
	env, dataHome := isolatedXDGEnv(t)
	projects := filepath.Join(dataHome, "ingot", "projects")

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, "run")
	cmd.Env = env
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting %s run: %v", bin, err)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-exited:
			case <-time.After(3 * time.Second):
				_ = cmd.Process.Kill()
			}
		}
	})

	select {
	case err := <-exited:
		t.Fatalf("ingot run exited early — err=%v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	case <-time.After(4 * time.Second):
	}

	wtype(t, "-k", "End", "-s", "250", "alpha", "-s", "250", "-k", "Return")
	if !pollForContent(t, projects, "alpha", 5*time.Second) {
		t.Fatalf("note never reached disk\nstderr:\n%s", stderr.String())
	}

	args := []string{"-k", "End"}
	for i := 0; i < tabsFromComposerToList; i++ {
		args = append(args, "-s", "250", "-k", "Tab")
	}
	args = append(args, "-s", "400", "-k", "space")
	wtype(t, args...)

	if !pollForContent(t, projects, "- [x] ", 5*time.Second) {
		t.Fatalf("Space did not mark the tab-focused note done — a row focused by Tab is invisible to "+
			"keymap.Nav and unselected, so the command had no target\nproject file:\n%s\nstderr:\n%s",
			readProjects(t, projects), stderr.String())
	}
}
