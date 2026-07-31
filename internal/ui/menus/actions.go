package menus

import (
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Accels is the accelerator table Register installs via
// SetAccelsForAction, and the same table
// AccelsForAction/ActionsForAccel round-trip against in this package's
// tests. Keys are bare action names — no "app." prefix, no target detail.
// An action absent here, or mapped to an empty slice, gets no
// accelerator (menu- or click-only).
var Accels = map[string][]string{
	"copy":            {"<Control>c"},
	"copy-as-list":    {"<Control><Shift>c"},
	"mark-done":       {"space"},
	"edit":            {"Return"},
	"edit-new-window": {"<Control>Return"},
	"merge":           {"<Control><Shift>m"},
	"shortcuts":       {"<Control>question"},
	"close":           {"<Control>w"},
}

// newSectionCustomID is the "custom" attribute value on the Move to
// submenu's "New Section..." item — see the package doc.
const newSectionCustomID = "menus-new-section"

// moveToCurrentSectionAction is the detailed action name the Move to
// submenu binds its current-section item to. It is never registered on
// the application: GTK's menu tracker renders an item insensitive when
// its bound action name does not exist, which is the only reliable way
// to get an unclickable-but-still-a-real-item row — an item with no
// action at all renders sensitive (see the package doc).
const moveToCurrentSectionAction = "app.move-to-current-section"

// Actions holds every gio.SimpleAction Register installs, so a popup
// controller can drive per-invocation enablement (Actions.Expand,
// Actions.Merge) and state (Actions.Project, Actions.KeepOnTop, and now
// Actions.ClearDone) that a menu model alone cannot express.
type Actions struct {
	Copy          *gio.SimpleAction
	CopyAsList    *gio.SimpleAction
	MarkDone      *gio.SimpleAction
	Expand        *gio.SimpleAction
	Edit          *gio.SimpleAction
	EditNewWindow *gio.SimpleAction
	Merge         *gio.SimpleAction
	MoveTo        *gio.SimpleAction
	Project       *gio.SimpleAction
	KeepOnTop     *gio.SimpleAction
	ClearDone     *gio.SimpleAction
	Shortcuts     *gio.SimpleAction
	Close         *gio.SimpleAction

	lastProjectAccelIDs []string
}

// Register installs every action Ingot's menus and accelerators need on
// app (the "app." prefix, so accelerators work with no menu open) and
// wires each to h. It reads h.CurrentProjectID and h.KeepOnTop once, as
// the "project" and "keep-on-top" actions' initial state. Merge starts
// disabled — see RefreshMergeEnablement — since its accelerator is
// app-global and reachable with no selection at all.
func Register(app *gtk.Application, h Handlers) *Actions {
	a := &Actions{}

	a.Copy = simpleCommand(app, "copy", h.Copy)
	a.CopyAsList = simpleCommand(app, "copy-as-list", h.CopyAsList)
	a.MarkDone = simpleCommand(app, "mark-done", h.MarkDone)
	a.Expand = simpleCommand(app, "expand", h.Expand)
	a.Edit = simpleCommand(app, "edit", h.Edit)
	a.EditNewWindow = simpleCommand(app, "edit-new-window", h.EditNewWindow)
	a.Merge = simpleCommand(app, "merge", h.Merge)
	a.Merge.SetEnabled(false)
	a.Shortcuts = simpleCommand(app, "shortcuts", h.Shortcuts)
	a.Close = simpleCommand(app, "close", h.Close)

	a.MoveTo = gio.NewSimpleAction("move-to", glib.NewVariantType("s"))
	a.MoveTo.ConnectActivate(func(param *glib.Variant) {
		if param != nil {
			h.MoveTo(param.String())
		}
	})
	app.AddAction(a.MoveTo)

	// clear-done is stateful (boolean), not merely parameterised, even
	// though the spec calls only "project" and "keep-on-top" stateful:
	// a stateless action's menu item always closes the popover on
	// activation (GTK's NORMAL item role), which would make a two-click
	// inline confirm impossible to complete. A boolean-state item gets
	// GTK's CHECK role instead, which keeps the popover open across both
	// activations — see the package doc.
	a.ClearDone = gio.NewSimpleActionStateful("clear-done", nil, glib.NewVariantBoolean(false))
	a.ClearDone.ConnectChangeState(func(value *glib.Variant) {
		armed := value.Boolean()
		if !armed && a.ClearDone.State().Boolean() {
			// false requested while previously true: the confirming
			// (second) activation.
			h.ClearDone()
		}
		a.ClearDone.SetState(value)
	})
	app.AddAction(a.ClearDone)

	a.Project = gio.NewSimpleActionStateful(
		"project", glib.NewVariantType("s"), glib.NewVariantString(h.CurrentProjectID()),
	)
	a.Project.ConnectChangeState(func(value *glib.Variant) {
		a.Project.SetState(value)
		h.SetProject(value.String())
	})
	app.AddAction(a.Project)

	a.KeepOnTop = gio.NewSimpleActionStateful("keep-on-top", nil, glib.NewVariantBoolean(h.KeepOnTop()))
	a.KeepOnTop.ConnectChangeState(func(value *glib.Variant) {
		a.KeepOnTop.SetState(value)
		h.SetKeepOnTop(value.Boolean())
	})
	app.AddAction(a.KeepOnTop)

	for name, accels := range Accels {
		if len(accels) == 0 {
			continue
		}
		app.SetAccelsForAction("app."+name, accels)
	}

	return a
}

// simpleCommand installs a stateless, parameterless action on app and
// routes its activation straight to fn, ignoring the (always-nil)
// activation parameter.
func simpleCommand(app *gtk.Application, name string, fn func()) *gio.SimpleAction {
	action := gio.NewSimpleAction(name, nil)
	action.ConnectActivate(func(*glib.Variant) { fn() })
	app.AddAction(action)
	return action
}

// SetProjectAccels (re)assigns Ctrl+1..Ctrl+9 to the "app.project"
// action, one accelerator per project in projectIDs order, matching the
// numbering BuildOverflow's project list renders. Projects past the
// ninth get no accelerator. Call this again whenever the project list
// changes order or membership — a's own record of the previous call's
// IDs is used to clear the accelerator of any project that dropped out,
// so a deleted or reordered project never keeps a stale Ctrl+N binding.
func (a *Actions) SetProjectAccels(app *gtk.Application, projectIDs []string) {
	const digits = "123456789"
	seen := make(map[string]bool, len(projectIDs))
	for i, id := range projectIDs {
		seen[id] = true
		detailed := detailedAction("project", id)
		if i >= len(digits) {
			app.SetAccelsForAction(detailed, nil)
			continue
		}
		app.SetAccelsForAction(detailed, []string{"<Control>" + string(digits[i])})
	}
	for _, id := range a.lastProjectAccelIDs {
		if seen[id] {
			continue
		}
		app.SetAccelsForAction(detailedAction("project", id), nil)
	}
	a.lastProjectAccelIDs = append([]string(nil), projectIDs...)
}

// RefreshMergeEnablement re-syncs Actions.Merge's enablement to the
// current selection size. Call this from the selection-changed handler
// as well as after every right-click — Merge Notes' Ctrl+Shift+M
// accelerator is reachable with no menu open, so per-click enablement
// alone leaves it stale between clicks.
func (a *Actions) RefreshMergeEnablement(selectionCount int) {
	a.Merge.SetEnabled(selectionCount >= 2)
}

// ClearDoneArmed reports whether the confirm state (the first
// activation) is currently in effect — a popover owner can use this to
// decide whether to relabel or otherwise highlight the row.
func (a *Actions) ClearDoneArmed() bool {
	return a.ClearDone.State().Boolean()
}

// ResetClearDone disarms the Clear Done confirmation without running
// Handlers.ClearDone. Call this whenever the overflow popover closes, so
// a stray re-open never inherits an armed confirm from a session the
// user abandoned.
func (a *Actions) ResetClearDone() {
	a.ClearDone.SetState(glib.NewVariantBoolean(false))
}

// detailedAction formats the GLib "detailed action name" for a
// string-parameterised action, quoting target through GVariant's own
// text-format printer so an ID containing a quote or other special
// character round-trips correctly.
func detailedAction(name, target string) string {
	return "app." + name + "(" + glib.NewVariantString(target).Print(true) + ")"
}
