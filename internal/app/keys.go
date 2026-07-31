package app

import (
	"log/slog"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/ui/keymap"
	"github.com/Yiin/ingot/internal/ui/notelist"
)

// wireListGate wires Space (mark done/undone) and BackSpace (delete) on
// the note list via keymap.InstallListGate rather than a
// gtk.Application accelerator: both are bare keys with no modifier, and
// an app-wide accelerator would fire even while the composer or search
// field is focused, eating the character the user meant to type.
// InstallListGate exists specifically to gate this correctly on
// IsTextFocused — see its own doc comment. Wrapped in safe(): this is a
// raw GTK key-controller callback gtkapp does not wrap in its own
// recover.
func (a *App) wireListGate() {
	keymap.InstallListGate(a.shell.List().ListView(), safe("mark-done-selected", a.markDoneSelected), safe("delete-selected", a.deleteSelected))
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
