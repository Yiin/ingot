// Package fsstore is the concrete, file-backed implementation of
// store.Store: an in-memory model of every project, loaded from
// $XDG_DATA_HOME/ingot/projects/*.md via mdfile.Parse, mutated
// synchronously in memory, and persisted back to disk on a debounced
// schedule. See New and Options for construction, and package store's
// doc comment for the single-goroutine threading rule every method here
// depends on.
//
// Setting Options.Watch turns on a background fsnotify watcher over
// Paths.Projects: an external change reloads the affected project (or,
// if the panel had pending edits, is preserved to Paths.Trash and
// overwritten — see reload.go). DeleteNotes, ClearDone, DeleteProject,
// and DeleteSection's note relocation each also write a trash file
// before they touch anything (trash.go), and DeleteNotes/ClearDone arm a
// single level of in-memory undo (undo.go).
package fsstore
