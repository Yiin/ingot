package app

import (
	"log/slog"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/ui/keymap"
	"github.com/Yiin/ingot/internal/ui/menus"
)

// wireMenus registers menus' actions on the application, attaches the
// note context menu to the list and the overflow menu to the search
// bar's overflow button. Must run after a.win and a.shell exist, and
// after applyKeyOverrides (called earlier, at the top of startup — see
// its own doc comment for why).
func (a *App) wireMenus() {
	a.menuActions = menus.Register(a.gapp.Application, a)

	// menus.Accels maps "mark-done" to bare "space" and "edit" to bare
	// "Return", and Register installs both as real gtk.Application
	// accelerators. A bare-key app-wide accelerator fires even while the
	// composer or search field is focused, eating the character the user
	// meant to type — precisely the failure mode keymap's own
	// ShouldGateForList policy exists to avoid, and why Space is already
	// routed through keymap.InstallListGate instead (see wireListGate).
	// Clearing the accelerator here leaves the action, and its menu
	// item, fully reachable by click; only the dangerous global
	// keybinding is removed. "edit" carries no real risk today (Edit is
	// a stub, see the Handlers methods below) but is cleared for the
	// same reason, so a future inline-edit feature does not inherit a
	// silent bug. applyMenuKeyOverrides (below) runs after this and may
	// re-enable either with the user's own, by-construction-validated
	// combo — that's an informed choice the user made, not the same risk
	// as an unconditional default.
	a.gapp.SetAccelsForAction("app.mark-done", nil)
	a.gapp.SetAccelsForAction("app.edit", nil)
	a.applyMenuKeyOverrides()

	a.wireExtraShortcuts()

	list := a.shell.List()
	a.ctxMenu = menus.NewContextMenuController(&a.win.Window, list.ListView(), a.menuActions, a, list, listSelection{list: list})
	a.ctxMenu.SetOnRebuilt(a.rebuildNewSectionEntry)

	menus.AttachOverflow(a.shell.SearchBar().OverflowButton(), a, a.menuActions)

	a.adapter.onProjectsChanged = a.refreshProjectAccels
	a.adapter.onActiveTitleChanged = a.onActiveProjectChanged
	a.refreshProjectAccels()
}

// onActiveProjectChanged keeps the composer's placeholder and the
// overflow menu's own "project" radio state in sync with whichever
// project the store now says is active — SetState, not Activate: it
// pushes the authoritative value without re-running Actions' own
// ConnectChangeState (which would call back into Handlers.SetProject
// and loop). Needed for any active-project change menus.Actions.Project
// didn't itself drive, chiefly cycleProject's Ctrl+Tab/Ctrl+Shift+Tab —
// without this, the overflow menu's checkmark goes stale the moment the
// user cycles projects by keyboard instead of picking one from the menu.
func (a *App) onActiveProjectChanged(title string) {
	a.shell.Composer().SetProject(title)
	if a.menuActions != nil {
		a.menuActions.Project.SetState(glib.NewVariantString(a.CurrentProjectID()))
	}
}

// keyOverrideActionAliases maps a config.toml [keys] action name to the
// real gio action name it should re-bind, for the one case where
// keymap.Table's own name and menus' installed action name genuinely
// differ: Table calls Ctrl+W's action "hide-panel" (matching its own
// Group Window wording); menus.Register installs the identical physical
// binding under "close" (Handlers.Close, BuildOverflow's Window > Close
// item — see menus.go). Every other overridable action's name already
// matches 1:1 between the two.
var keyOverrideActionAliases = map[string]string{
	"hide-panel": "close",
}

// applyKeyOverrides validates config.toml's [keys] section against
// keymap.Table (an override naming an action Table has no entry for is
// warned and dropped — keymap.ByAction is the validation authority per
// action name, see config.Config.Keys' own doc comment on why
// config.Load itself cannot do this check) and applies the rest via
// keymap.ApplyOverrides, which does its own further rejection (bad
// accelerator syntax, a same-scope collision, a ScopeGlobal target —
// see its own doc comment) and is warned here too. The survivors are
// kept on a.keyOverrides for applyMenuKeyOverrides (wireMenus, which
// needs Register to have already run) and are already live in Table
// itself by the time this returns, for every caller that reads
// accelerators from there — bindTableAction (wireCopyShortcuts,
// wireExtraShortcuts) and keymap.Resolve (InstallNav) alike. Must
// therefore run before all of those, at the very top of startup.
func (a *App) applyKeyOverrides() {
	candidates := make(map[string]string, len(a.cfg.Keys))
	for action, accel := range a.cfg.Keys {
		if _, ok := keymap.ByAction(action); !ok {
			slog.Warn("app: config.toml [keys]: unknown action, ignored", "action", action)
			continue
		}
		if accel != "" {
			candidates[action] = accel
		}
	}

	rejected := keymap.ApplyOverrides(candidates)
	for action, reason := range rejected {
		slog.Warn("app: config.toml [keys]: override rejected", "action", action, "reason", reason)
		delete(candidates, action)
	}
	a.keyOverrides = candidates
}

// applyMenuKeyOverrides re-issues SetAccelsForAction for every accepted
// override (see applyKeyOverrides) naming an action menus.Register
// itself installs a real accelerator for — menus.Accels is a separate
// table from keymap.Table, so an override that already landed in Table
// does not reach these on its own. keyOverrideActionAliases covers the
// one name mismatch between the two tables.
func (a *App) applyMenuKeyOverrides() {
	for action, accel := range a.keyOverrides {
		target := action
		if alias, ok := keyOverrideActionAliases[action]; ok {
			target = alias
		}
		if _, ok := menus.Accels[target]; ok {
			a.gapp.SetAccelsForAction("app."+target, []string{accel})
		}
	}
}

// bindTableAction binds action's current keymap.Table accelerators
// (reflecting any config.toml [keys] override already applied by
// applyKeyOverrides) as a real gtk.Application accelerator running fn.
// Panics if action names no Table entry — every caller here names a
// real, known ScopeApp entry, so a mismatch is a programming error to
// catch immediately, not a runtime condition.
func (a *App) bindTableAction(action string, fn func()) {
	e, ok := keymap.ByAction(action)
	if !ok {
		panic("app: bindTableAction: unknown keymap action " + action)
	}
	a.bindAction(action, e.Accels, fn)
}

// refreshProjectAccels re-syncs the overflow menu's Ctrl+1..9 project
// accelerators to the store's current project order. Call at startup and
// whenever store.ProjectListChanged fires.
func (a *App) refreshProjectAccels() {
	refs := a.store.Projects()
	ids := make([]string, len(refs))
	for i, p := range refs {
		ids[i] = string(p.ID)
	}
	a.menuActions.SetProjectAccels(a.gapp.Application, ids)
}

// rebuildNewSectionEntry re-adds the Move to submenu's inline "New
// Section..." entry after every context-menu rebuild — GTK discards a
// popover's custom children each time SetMenuModel replaces its model,
// so this must run on every right-click, not just once (see menus'
// package doc). Each rebuild gets a fresh Entry: reusing one across
// rebuilds would try to re-add a widget GTK already dropped.
func (a *App) rebuildNewSectionEntry(popover *gtk.PopoverMenu) {
	entry := gtk.NewEntry()
	entry.SetPlaceholderText("New Section...")
	entry.ConnectActivate(func() {
		defer guard("new-section-entry")()
		title := strings.TrimSpace(entry.Text())
		if title == "" {
			return
		}
		a.NewSection(a.CurrentProjectID(), title)
		popover.Popdown()
	})
	popover.AddChild(entry, menus.NewSectionCustomID)
}

// The following methods implement menus.Handlers.

func (a *App) Copy()          { a.copySelection(false, false) }
func (a *App) CopyAsList()    { a.copySelection(true, false) }
func (a *App) MarkDone()      { a.markDoneSelected() }
func (a *App) Expand()        {} // no note editor or row-expand feature yet — see the child spec's stub allowance
func (a *App) Edit()          {} // ditto: copper-l2z.27's inline editor doesn't exist yet
func (a *App) EditNewWindow() {} // ditto

// Merge combines the current selection into one note, in document
// order, at the position of the first. A no-op with fewer than two
// selected — store.MergeNotes itself would reject that, but Actions
// already keeps the Merge action disabled below two, so this only ever
// runs reachable from a real selection.
func (a *App) Merge() {
	selected := a.shell.List().Selected()
	if len(selected) < 2 {
		return
	}
	ids := make([]store.NoteID, len(selected))
	for i, it := range selected {
		ids[i] = store.NoteID(it.ID)
	}
	if _, err := a.store.MergeNotes(ids); err != nil {
		slog.Warn("app: merge notes", "err", err)
	}
}

// MoveTo relocates the current selection into an existing section or
// project, per BuildMoveToSubmenu's "section:<id>"/"project:<id>"
// target encoding (see the menus package doc).
func (a *App) MoveTo(target string) {
	sectionID, ok := a.resolveMoveTarget(target)
	if !ok {
		return
	}
	selected := a.shell.List().Selected()
	if len(selected) == 0 {
		return
	}
	ids := make([]store.NoteID, len(selected))
	for i, it := range selected {
		ids[i] = store.NoteID(it.ID)
	}
	if err := a.store.MoveNotes(ids, sectionID); err != nil {
		slog.Warn("app: move notes", "err", err)
	}
}

func (a *App) resolveMoveTarget(target string) (store.SectionID, bool) {
	kind, id, ok := strings.Cut(target, ":")
	if !ok {
		return "", false
	}
	switch kind {
	case "section":
		return store.SectionID(id), true
	case "project":
		proj, err := a.store.Project(store.ProjectID(id))
		if err != nil || len(proj.Sections) == 0 {
			return "", false
		}
		return proj.Sections[0].ID, true
	default:
		return "", false
	}
}

// NewSection creates a new section in projectID and, if the current
// selection is non-empty, moves it there — the point of reaching this
// from the Move to submenu's own inline entry (see
// rebuildNewSectionEntry) rather than a general "add section" affordance
// elsewhere.
func (a *App) NewSection(projectID, title string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	sectionID, err := a.store.AddSection(store.ProjectID(projectID), title)
	if err != nil {
		slog.Warn("app: new section", "err", err)
		return
	}
	selected := a.shell.List().Selected()
	if len(selected) == 0 {
		return
	}
	ids := make([]store.NoteID, len(selected))
	for i, it := range selected {
		ids[i] = store.NoteID(it.ID)
	}
	if err := a.store.MoveNotes(ids, sectionID); err != nil {
		slog.Warn("app: new section: move notes", "err", err)
	}
}

func (a *App) SetProject(id string) {
	if err := a.store.SetActive(store.ProjectID(id)); err != nil {
		slog.Warn("app: set project", "err", err)
	}
}

// SetKeepOnTop persists the overflow menu's Keep on Top preference.
//
// Nothing applies it yet. Wayland has no client-side "keep above" — the
// old gtk_window_set_keep_above was X11-only and GTK4 dropped it — so
// staying on top is a compositor decision, reachable only through a
// window rule against the "lt.yiin.ingot" app id. This keeps the menu's
// own checkmark real and durable across a restart without fabricating a
// behaviour the panel does not have. See copper-4tn.
func (a *App) SetKeepOnTop(on bool) {
	a.panelState.KeepOnTop = on
	a.savePanelState()
}

func (a *App) ClearDone() {
	if err := a.store.ClearDone(); err != nil {
		slog.Warn("app: clear done", "err", err)
	}
}

func (a *App) Shortcuts() {
	win := keymap.NewShortcutsWindow()
	win.SetTransientFor(&a.win.Window)
	win.Present()
}

func (a *App) Close() { a.hide() }

func (a *App) RowIsTruncated(idx int) bool { return a.shell.List().IsTruncatedAt(idx) }

func (a *App) RowIsDone(idx int) bool {
	it := a.shell.List().ItemAtViewPosition(idx)
	return it != nil && it.Done
}

// RowIsExpanded always reports false: there is no expand/collapse
// feature behind it yet (see Expand's own stub note above).
func (a *App) RowIsExpanded(idx int) bool { return false }

func (a *App) SelectionCount() int { return len(a.shell.List().Selected()) }

func (a *App) CurrentProjectID() string { return string(a.store.Active()) }

// CurrentSectionID answers the section of whichever row the context
// menu currently targets (a.ctxMenu.Target(), set by
// ContextMenuController immediately before this and the rest of
// ContextInfo are read — see its onPressed) rather than some notion of
// a single app-wide "current" section, since a project can have several.
func (a *App) CurrentSectionID() string {
	if a.ctxMenu == nil {
		return ""
	}
	it := a.shell.List().ItemAtViewPosition(a.ctxMenu.Target())
	if it == nil {
		return ""
	}
	return it.SectionID
}

func (a *App) Sections() []menus.Section {
	proj, err := a.store.Project(a.store.Active())
	if err != nil {
		return nil
	}
	out := make([]menus.Section, len(proj.Sections))
	for i, s := range proj.Sections {
		out[i] = menus.Section{ID: string(s.ID), Title: s.Title}
	}
	return out
}

func (a *App) Projects() []menus.Project {
	refs := a.store.Projects()
	out := make([]menus.Project, len(refs))
	for i, p := range refs {
		out[i] = menus.Project{ID: string(p.ID), Title: p.Title}
	}
	return out
}

func (a *App) KeepOnTop() bool { return a.panelState.KeepOnTop }

var _ menus.Handlers = (*App)(nil)
