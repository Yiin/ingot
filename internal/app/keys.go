package app

import (
	"log/slog"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/ui/keymap"
	"github.com/Yiin/ingot/internal/ui/notelist"
)

// wireListGate wires every keymap.Table ScopeList action that operates
// on the focused or selected note, through keymap.InstallListGate rather
// than gtk.Application accelerators.
//
// The gate is what makes bare keys safe. Space, Return, Left and Right
// all mean something entirely different while the composer or the search
// field has focus, and an app-wide accelerator fires regardless of focus
// — so binding them that way would eat the character the user meant to
// type. InstallListGate resolves against Table and only acts while no
// text widget is focused; see its own doc comment.
//
// Ctrl+Return is here for the same reason even though it carries a
// modifier: the composer commits on Ctrl+Enter (composer.installCommitKeys),
// so an app-wide binding would open an editor window instead of saving
// the note being typed. wireMenus clears the accelerator menus.Accels
// installs for it, exactly as it already does for edit and mark-done.
//
// Every handler is wrapped in safe(): these are raw GTK key-controller
// callbacks, which gtkapp does not wrap in its own recover.
func (a *App) wireListGate() {
	keymap.InstallListGate(a.shell.List().ListView(), a.listActionHandlers())
}

// listActionHandlers is wireListGate's map, split out so
// TestEveryListShortcutIsWired can read which keymap.Table actions this
// package actually implements without needing a display. Building the map
// touches no App field, so the test can call it on a bare &App{}.
func (a *App) listActionHandlers() map[string]func() {
	return map[string]func(){
		"mark-done":       safe("mark-done-selected", a.markDoneSelected),
		"delete-note":     safe("delete-selected", a.deleteSelected),
		"edit-inline":     safe("edit-inline", a.Edit),
		"edit-new-window": safe("edit-new-window", a.EditNewWindow),
		"expand":          safe("expand", func() { a.setFocusedExpanded(true) }),
		"collapse":        safe("collapse", func() { a.setFocusedExpanded(false) }),
		"toggle-expand":   safe("toggle-expand", a.Expand),
	}
}

// wireListToggle persists a row checkbox click — the click already
// updated the Item and the widget optimistically (see
// notelist.List.ConnectToggled's own doc); this is what makes it stick
// across a reload instead of silently reverting. Wrapped in safe(): a
// raw GTK signal callback gtkapp does not wrap in its own recover.
func (a *App) wireListToggle() {
	a.shell.List().ConnectToggled(func(it *notelist.Item, done bool) {
		defer guard("list-toggled")()
		if err := a.store.SetNoteDone(store.NoteID(it.ID), done); err != nil {
			slog.Warn("app: mark done", "id", it.ID, "err", err)
		}
	})
}

// markDoneSelected toggles every selected note's own done state — not a
// single "set all done", so a mixed selection acts the same way
// clicking each row's own checkbox individually would.
func (a *App) markDoneSelected() {
	for _, it := range a.shell.List().Selected() {
		if err := a.store.SetNoteDone(store.NoteID(it.ID), !it.Done); err != nil {
			slog.Warn("app: mark done", "id", it.ID, "err", err)
		}
	}
}

// deleteSelected removes every selected note in one call, so fsstore
// emits one NotesSpliced removal per contiguous run rather than one per
// note.
func (a *App) deleteSelected() {
	selected := a.shell.List().Selected()
	if len(selected) == 0 {
		return
	}
	ids := make([]store.NoteID, len(selected))
	for i, it := range selected {
		ids[i] = store.NoteID(it.ID)
	}
	if err := a.store.DeleteNotes(ids); err != nil {
		slog.Warn("app: delete notes", "err", err)
		return
	}
	a.shell.RefreshEmptyState("", 0)
}

// wireExtraShortcuts installs four of keymap.Table's ScopeApp entries
// that have no menus.Handlers counterpart: focus-composer and undo
// (redo has no binding at all — store.Store keeps a single undo slot,
// not a stack, so there is nothing for a "redo" to reverse), and
// next-project/previous-project, which cycle store.Projects() order
// rather than jumping to one by id the way menus' own Ctrl+1..9
// (SetProjectAccels) does. Not exhaustive: focus-search (already wired
// independently, inside searchbar's own installShortcuts) and
// clear-done (deliberately left with no accelerator at all — see
// menus.Accels' own omission and its reasoning) are Table entries too,
// just not this function's concern.
func (a *App) wireExtraShortcuts() {
	a.bindTableAction("focus-composer", func() { a.shell.Composer().Focus() })
	a.bindTableAction("undo", func() {
		if err := a.store.Undo(); err != nil {
			slog.Warn("app: undo", "err", err)
		}
	})
	a.bindTableAction("next-project", func() { a.cycleProject(1) })
	a.bindTableAction("previous-project", func() { a.cycleProject(-1) })
}

// cycleProject switches the active project delta positions forward (1)
// or back (-1) through store.Projects() order, wrapping around either
// end. A no-op if there are no projects (should not happen: startup
// seeds one) or the active project is somehow not among them.
func (a *App) cycleProject(delta int) {
	refs := a.store.Projects()
	if len(refs) == 0 {
		return
	}
	active := a.store.Active()
	idx := -1
	for i, p := range refs {
		if p.ID == active {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	next := (idx + delta + len(refs)) % len(refs)
	if err := a.store.SetActive(refs[next].ID); err != nil {
		slog.Warn("app: cycle project", "err", err)
	}
}
