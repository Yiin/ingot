// Package editorwindow implements the note editor window: the second
// half of copper-l2z.27, alongside internal/ui/widget's inline row
// editing.
//
// Ingot is not single-window. Alongside the always-on-top panel, a note
// can also open in an ordinary toplevel GtkWindow — not a layer-shell
// surface, so the compositor tiles and focuses it like any other window
// — for editing a long note that the panel's 3-line-clamped row can't
// comfortably hold. There is no OK/Cancel: the window saves itself on a
// debounce after every keystroke and unconditionally on close.
//
// Manager owns every currently-open editor window, keyed by note ID:
// opening a note that already has one presents that window instead of a
// second one. Window is deliberately unaware of internal/store or
// internal/ui/notelist — it takes a plain (id, title, body) triple and
// reports edits back through a callback, the same decoupling
// internal/ui/notelist's own Item keeps from a future store.Note.
package editorwindow
