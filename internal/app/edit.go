package app

import (
	"log/slog"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/ui/editorwindow"
)

// wireNoteEditing connects the two places a note's body can be rewritten
// — the row's own inline editor and a standalone editor window — to the
// store, and to each other.
//
// The same note can be open in both at once, so both have to write. The
// editor window saves on a 400ms debounce and on close; the inline editor
// commits on Enter. Whichever fires, the store is written, and the other
// editor is told directly so it cannot sit on a stale body and later save
// it back over the newer one.
//
// Only the editors need telling directly. The panel row behind them
// already updates on its own: SetNoteBody emits store.NoteUpdated, which
// storeAdapter.onUpdated turns into a model refresh. Writing the row here
// as well would just race that.
func (a *App) wireNoteEditing() {
	a.editors = editorwindow.NewManager(func(id, text string) {
		defer guard("editor-window-save")()
		a.saveNoteBody(id, text)
	})

	a.shell.List().ConnectEditCommitted(func(id, text string) {
		defer guard("inline-edit-committed")()
		a.saveNoteBody(id, text)
		// StartInlineEdit has already updated the Item and the row label
		// itself, so only the editor window still needs telling. A no-op
		// unless this note also has one open.
		a.editors.UpdateBody(id, text)
	})
}

// saveNoteBody persists one note's new body. A failure warns rather than
// propagating: both callers are UI callbacks with nowhere to return an
// error to, and the text is still on screen for the user to retry.
func (a *App) saveNoteBody(id, text string) {
	if err := a.store.SetNoteBody(store.NoteID(id), text); err != nil {
		slog.Warn("app: save note body", "id", id, "err", err)
	}
}
