package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/Yiin/ingot/internal/clipboard"
	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/clipfmt"
)

// clipboardWriteTimeout bounds the clipboard-write worker — generous
// over clipboard.WlCopyWriter's own 2s internal timeout, same reasoning
// as primaryReadTimeout.
const clipboardWriteTimeout = 3 * time.Second

// wireCopyShortcuts installs "Copy as List, and Mark Done" and Quit as
// app-wide accelerators. Both use modifier-bearing accelerators, so —
// unlike mark-done's own bare Space, wired separately via
// keymap.InstallListGate (see keys.go) — neither can shadow ordinary
// typing in the composer or search field.
//
// Copy and Copy as List themselves are not bound here: menus.Register
// (wireMenus, which runs after this) installs both as "app.copy" and
// "app.copy-as-list" via Handlers.Copy/CopyAsList (implemented in
// menus.go as a.copySelection(false, false) and a.copySelection(true,
// false)) — registering them a second time under the same name here
// would collide. Likewise Close/hide-panel: menus' "close" action
// already claims Ctrl+W via Handlers.Close, so this package has only one
// name for it, not two.
func (a *App) wireCopyShortcuts() {
	// "Do not copy that behaviour [Copy as List auto-marking notes
	// done]; offer Copy as List and Mark Done as a separate action
	// instead" — the child spec's own words. A copy alone never
	// mutates; this is the separate, explicit action. Not a
	// keymap.Table action (it's an Ingot-specific extra beyond the base
	// spec), so — unlike "quit" below — its accelerator isn't
	// overridable via config.toml's [keys] section.
	a.bindAction("copy-as-list-and-done", []string{"<Control><Alt><Shift>c"}, func() { a.copySelection(true, true) })
	a.bindTableAction("quit", func() { a.shutdown() })
}

// bindAction registers a named, parameterless action via gtkapp.App
// (which wraps it in its own panic recovery) and gives it accels as a
// real gtk.Application accelerator.
func (a *App) bindAction(name string, accels []string, fn func()) {
	a.gapp.AddAction(name, fn)
	a.gapp.SetAccelsForAction("app."+name, accels)
}

// copySelection renders the selected notes (Copy or Copy as List) and
// writes them to CLIPBOARD via a worker goroutine — WlCopyWriter shells
// out and can block for up to 2s, which would freeze the panel if done
// inline. markDone additionally marks every selected note done, only
// after the clipboard write itself succeeds, and only for the dedicated
// "Copy as List and Mark Done" action — see the doc comment above.
func (a *App) copySelection(asList, markDone bool) {
	selected := a.shell.List().Selected()
	if len(selected) == 0 {
		return
	}

	ids := make([]store.NoteID, 0, len(selected))
	notes := make([]store.Note, 0, len(selected))
	for _, it := range selected {
		id := store.NoteID(it.ID)
		n, err := a.store.Note(id)
		if err != nil {
			continue
		}
		ids = append(ids, id)
		notes = append(notes, n)
	}
	if len(notes) == 0 {
		return
	}

	var text, doneMsg string
	if asList {
		text = clipfmt.CopyAsList(notes)
		doneMsg = "Copied as List"
	} else {
		text = clipfmt.Copy(notes)
		doneMsg = "Copied"
	}

	writer := clipboard.NewWriter(nil)
	goSafe("clipboard-write", func() {
		ctx, cancel := context.WithTimeout(context.Background(), clipboardWriteTimeout)
		defer cancel()
		err := writer.SetText(ctx, text)
		a.gapp.Post(func() {
			if err != nil {
				slog.Warn("app: copy: write clipboard", "err", err)
				return
			}
			a.notifier().Message(doneMsg)
			if !markDone {
				return
			}
			for _, id := range ids {
				if err := a.store.SetNoteDone(id, true); err != nil {
					slog.Warn("app: copy-as-list-and-done: mark done", "id", id, "err", err)
				}
			}
		})
	})
}
