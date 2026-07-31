package app

import (
	"testing"

	"github.com/Yiin/ingot/internal/config"
	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/ui/gtkapp"
	"github.com/Yiin/ingot/internal/ui/keymap"
)

// TestApp_ResolveMoveTarget exercises resolveMoveTarget's own store
// access (Project lookup for a "project:<id>" target) against a real
// fsstore, with no GTK involved — Merge/MoveTo/NewSection themselves
// additionally touch a.shell.List(), a live GTK widget this package's
// non-integration tests cannot build (see the codebase's own convention,
// e.g. keymap.NewShortcutsWindow's doc comment), so only the pure
// resolution step is covered here.
func TestApp_ResolveMoveTarget(t *testing.T) {
	st := newTestStore(t)
	a := &App{store: st}

	activeID, err := st.CreateProject("Notes")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	proj, err := st.Project(activeID)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	firstSection := proj.Sections[0].ID

	otherID, err := st.CreateProject("Other")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	otherProj, err := st.Project(otherID)
	if err != nil {
		t.Fatalf("Project(other): %v", err)
	}

	cases := []struct {
		name   string
		target string
		want   store.SectionID
		wantOK bool
	}{
		{"section target", "section:" + string(firstSection), firstSection, true},
		{"project target resolves to its first section", "project:" + string(otherID), otherProj.Sections[0].ID, true},
		{"unknown project", "project:does-not-exist", "", false},
		{"malformed target with no colon", "garbage", "", false},
		{"unknown kind", "note:abc", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := a.resolveMoveTarget(c.target)
			if ok != c.wantOK {
				t.Fatalf("resolveMoveTarget(%q) ok = %v, want %v", c.target, ok, c.wantOK)
			}
			if ok && got != c.want {
				t.Errorf("resolveMoveTarget(%q) = %q, want %q", c.target, got, c.want)
			}
		})
	}
}

func TestApp_CycleProject(t *testing.T) {
	st := newTestStore(t)
	a := &App{store: st}

	first, err := st.CreateProject("First")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	second, err := st.CreateProject("Second")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	third, err := st.CreateProject("Third")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	// CreateProject does not itself change the active project.
	if st.Active() != first {
		t.Fatalf("Active = %q, want unchanged %q", st.Active(), first)
	}

	a.cycleProject(1)
	if st.Active() != second {
		t.Fatalf("after +1: Active = %q, want %q", st.Active(), second)
	}
	a.cycleProject(1)
	if st.Active() != third {
		t.Fatalf("after +1: Active = %q, want %q", st.Active(), third)
	}
	a.cycleProject(1)
	if st.Active() != first {
		t.Fatalf("cycling past the last project should wrap to the first: Active = %q, want %q", st.Active(), first)
	}
	a.cycleProject(-1)
	if st.Active() != third {
		t.Fatalf("cycling back past the first project should wrap to the last: Active = %q, want %q", st.Active(), third)
	}
}

// TestApp_ApplyKeyOverrides is the app-side half of the fix for a config
// override crashing the app on the very first list keypress (an invalid
// or colliding accelerator reaching keymap.Table unvalidated): it checks
// applyKeyOverrides actually rejects what keymap.ApplyOverrides itself
// rejects (an unknown action, and a ScopeGlobal target — see
// keymap.ApplyOverrides' own doc comment) rather than accepting
// everything config.toml happened to name.
func TestApp_ApplyKeyOverrides(t *testing.T) {
	saved := make([][]string, len(keymap.Table))
	for i, e := range keymap.Table {
		saved[i] = e.Accels
	}
	t.Cleanup(func() {
		for i, accels := range saved {
			keymap.Table[i].Accels = accels
		}
	})

	a := &App{
		gapp: gtkapp.New("lt.yiin.ingot.test-key-overrides"),
		cfg: config.Config{Keys: map[string]string{
			"quit":           "<Control><Shift>q",
			"no-such-action": "<Control>x",
			"global-capture": "<Control>g",
		}},
	}

	a.applyKeyOverrides()

	if got := a.keyOverrides["quit"]; got != "<Control><Shift>q" {
		t.Errorf(`keyOverrides["quit"] = %q, want "<Control><Shift>q" (valid, non-colliding, ScopeApp)`, got)
	}
	if _, ok := a.keyOverrides["no-such-action"]; ok {
		t.Error(`keyOverrides["no-such-action"] present, want rejected (unknown action)`)
	}
	if _, ok := a.keyOverrides["global-capture"]; ok {
		t.Error(`keyOverrides["global-capture"] present, want rejected (ScopeGlobal has no GTK accelerator to override)`)
	}

	e, ok := keymap.ByAction("quit")
	if !ok || len(e.Accels) != 1 || e.Accels[0] != "<Control><Shift>q" {
		t.Errorf("keymap.Table's quit Accels = %v, want the override applied", e.Accels)
	}
	if e, _ := keymap.ByAction("global-capture"); len(e.Accels) != 0 {
		t.Errorf("keymap.Table's global-capture Accels = %v, want unchanged (empty)", e.Accels)
	}
}

func TestApp_BindTableAction_PanicsOnUnknownAction(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("bindTableAction did not panic on an unknown action")
		}
	}()
	a := &App{gapp: gtkapp.New("lt.yiin.ingot.test-bind-panic")}
	a.bindTableAction("not-a-real-action", func() {})
}
