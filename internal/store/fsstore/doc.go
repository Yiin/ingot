// Package fsstore is the concrete, file-backed implementation of
// store.Store: an in-memory model of every project, loaded from
// $XDG_DATA_HOME/ingot/projects/*.md via mdfile.Parse, mutated
// synchronously in memory, and persisted back to disk on a debounced
// schedule. See New and Options for construction, and package store's
// doc comment for the single-goroutine threading rule every method here
// depends on.
//
// File watching, conflict resolution, trash, and undo are out of scope
// here — they land in a later package addition. Options.Watch is
// accepted and ignored.
package fsstore
