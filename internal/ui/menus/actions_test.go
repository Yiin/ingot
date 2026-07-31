package menus

import (
	"strings"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func newTestApp(t *testing.T) *gtk.Application {
	t.Helper()
	return gtk.NewApplication("lt.yiin.ingot.test", gio.ApplicationFlagsNone)
}

// TestAccelsRoundTrip checks that every accel Register installs comes
// back out of AccelsForAction, and that ActionsForAccel finds the action
// again from at least one of the accels AccelsForAction reports — never
// by comparing accel strings directly, since GTK normalises modifier
// order for display (see the package doc).
func TestAccelsRoundTrip(t *testing.T) {
	app := newTestApp(t)
	h := newFakeHandlers()
	Register(app, h)

	for name, want := range Accels {
		if len(want) == 0 {
			continue
		}
		got := app.AccelsForAction("app." + name)
		if len(got) != len(want) {
			t.Errorf("action %q: AccelsForAction returned %v, want same length as %v", name, got, want)
			continue
		}
		for _, accel := range got {
			actions := app.ActionsForAccel(accel)
			if !containsString(actions, "app."+name) {
				t.Errorf("action %q: ActionsForAccel(%q) = %v, want to contain %q", name, accel, actions, "app."+name)
			}
		}
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestSimpleCommandsActivateHandlers(t *testing.T) {
	app := newTestApp(t)
	h := newFakeHandlers()
	a := Register(app, h)

	// Merge is excluded here: it starts disabled (see
	// TestMergeStartsDisabled) and Activate is a no-op on a disabled
	// action.
	cases := []struct {
		action *gio.SimpleAction
		want   string
	}{
		{a.Copy, "copy"},
		{a.CopyAsList, "copy-as-list"},
		{a.MarkDone, "mark-done"},
		{a.Expand, "expand"},
		{a.Edit, "edit"},
		{a.EditNewWindow, "edit-new-window"},
		{a.Shortcuts, "shortcuts"},
		{a.Close, "close"},
	}
	for _, c := range cases {
		h.calls = nil
		c.action.Activate(nil)
		if len(h.calls) != 1 || h.calls[0] != c.want {
			t.Errorf("activating %q recorded calls %v, want [%q]", c.want, h.calls, c.want)
		}
	}
}

func TestMoveToPassesTargetThrough(t *testing.T) {
	app := newTestApp(t)
	h := newFakeHandlers()
	a := Register(app, h)

	a.MoveTo.Activate(glib.NewVariantString("section:abc"))
	if h.lastMoveTo != "section:abc" {
		t.Errorf("MoveTo target = %q, want %q", h.lastMoveTo, "section:abc")
	}
}

// TestMergeStartsDisabledAndEnablesWithSelection checks Merge's global
// accelerator cannot fire with fewer than two notes selected, and that
// RefreshMergeEnablement is what makes it reachable again — see the
// package doc's note on Merge's app-global accelerator.
func TestMergeStartsDisabledAndEnablesWithSelection(t *testing.T) {
	app := newTestApp(t)
	h := newFakeHandlers()
	a := Register(app, h)

	if a.Merge.Enabled() {
		t.Fatal("Merge should start disabled")
	}
	a.Merge.Activate(nil)
	if len(h.calls) != 0 {
		t.Fatalf("activating disabled Merge called handlers %v, want none", h.calls)
	}

	a.RefreshMergeEnablement(1)
	if a.Merge.Enabled() {
		t.Fatal("Merge should stay disabled with only one note selected")
	}

	a.RefreshMergeEnablement(2)
	if !a.Merge.Enabled() {
		t.Fatal("Merge should be enabled with two notes selected")
	}
	a.Merge.Activate(nil)
	if len(h.calls) != 1 || h.calls[0] != "merge" {
		t.Fatalf("activating enabled Merge called handlers %v, want [merge]", h.calls)
	}
}

// TestClearDoneRequiresTwoActivations checks the boolean-state design
// documented in doc.go: the first activation arms the confirm (state
// becomes true) without calling Handlers.ClearDone; the second (state
// back to false) is read as the confirming click.
func TestClearDoneRequiresTwoActivations(t *testing.T) {
	app := newTestApp(t)
	h := newFakeHandlers()
	a := Register(app, h)

	if a.ClearDone.State().Boolean() {
		t.Fatal("clear-done should start unarmed")
	}

	a.ClearDone.Activate(nil)
	if len(h.calls) != 0 {
		t.Fatalf("first clear-done activation called handlers %v, want none", h.calls)
	}
	if !a.ClearDoneArmed() {
		t.Fatal("first clear-done activation did not arm the confirmation")
	}

	a.ClearDone.Activate(nil)
	if len(h.calls) != 1 || h.calls[0] != "clear-done" {
		t.Fatalf("second clear-done activation called handlers %v, want [clear-done]", h.calls)
	}
	if a.ClearDoneArmed() {
		t.Fatal("clear-done still armed after the confirming activation")
	}
}

func TestClearDoneResetDisarms(t *testing.T) {
	app := newTestApp(t)
	h := newFakeHandlers()
	a := Register(app, h)

	a.ClearDone.Activate(nil)
	if !a.ClearDoneArmed() {
		t.Fatal("expected armed after first activation")
	}
	a.ResetClearDone()
	if a.ClearDoneArmed() {
		t.Fatal("expected disarmed after ResetClearDone")
	}
	if len(h.calls) != 0 {
		t.Fatalf("ResetClearDone called handlers %v, want none", h.calls)
	}

	// Disarmed means the next activation re-arms rather than confirms.
	a.ClearDone.Activate(nil)
	if len(h.calls) != 0 {
		t.Fatalf("activation after reset called handlers %v, want none (re-armed, not confirmed)", h.calls)
	}
	if !a.ClearDoneArmed() {
		t.Fatal("activation after reset should re-arm")
	}
}

func TestProjectStateChangesAndCallsHandler(t *testing.T) {
	app := newTestApp(t)
	h := newFakeHandlers()
	h.projectID = "proj-1"
	a := Register(app, h)

	if got := a.Project.State().String(); got != "proj-1" {
		t.Fatalf("initial project state = %q, want %q", got, "proj-1")
	}

	a.Project.Activate(glib.NewVariantString("proj-2"))
	if got := a.Project.State().String(); got != "proj-2" {
		t.Errorf("project state after activate = %q, want %q", got, "proj-2")
	}
	if h.lastSetProject != "proj-2" {
		t.Errorf("SetProject called with %q, want %q", h.lastSetProject, "proj-2")
	}
}

func TestKeepOnTopTogglesAndCallsHandler(t *testing.T) {
	app := newTestApp(t)
	h := newFakeHandlers()
	h.keepOnTop = false
	a := Register(app, h)

	if a.KeepOnTop.State().Boolean() {
		t.Fatal("initial keep-on-top state should be false")
	}

	a.KeepOnTop.Activate(nil)
	if !a.KeepOnTop.State().Boolean() {
		t.Error("keep-on-top state should be true after activating with no parameter")
	}
	if !h.lastSetKeepOnTop {
		t.Error("SetKeepOnTop should have been called with true")
	}
}

func TestSetProjectAccelsNumbersUpToNine(t *testing.T) {
	app := newTestApp(t)
	h := newFakeHandlers()
	a := Register(app, h)

	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	a.SetProjectAccels(app, ids)

	for i, id := range ids[:9] {
		detailed := detailedAction("project", id)
		got := app.AccelsForAction(detailed)
		want := "<Control>" + string("123456789"[i])
		if len(got) != 1 {
			t.Fatalf("project %q: AccelsForAction = %v, want one accel", id, got)
		}
		back := app.ActionsForAccel(got[0])
		found := false
		for _, action := range back {
			if strings.HasPrefix(action, "app.project") {
				found = true
			}
		}
		if !found {
			t.Errorf("project %q: accel %q resolves to %v, want an app.project action", id, want, back)
		}
	}

	if got := app.AccelsForAction(detailedAction("project", "j")); len(got) != 0 {
		t.Errorf("tenth project got an accel %v, want none", got)
	}
}

// TestSetProjectAccelsClearsStaleAccel checks that a project dropped
// from the list on a later call loses its old Ctrl+N binding, rather
// than keeping it alongside whatever project now occupies that slot.
func TestSetProjectAccelsClearsStaleAccel(t *testing.T) {
	app := newTestApp(t)
	h := newFakeHandlers()
	a := Register(app, h)

	a.SetProjectAccels(app, []string{"a", "b"})
	if got := app.AccelsForAction(detailedAction("project", "b")); len(got) != 1 {
		t.Fatalf("project b initial accel = %v, want one accel", got)
	}

	a.SetProjectAccels(app, []string{"a"})
	if got := app.AccelsForAction(detailedAction("project", "b")); len(got) != 0 {
		t.Errorf("dropped project b still has accel %v, want none", got)
	}
}
