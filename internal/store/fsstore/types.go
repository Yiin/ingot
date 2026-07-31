package fsstore

import (
	"time"

	"github.com/Yiin/ingot/internal/store"
)

// projectEntry is everything the Store tracks about one project beyond
// the store.Project value itself: where it lives on disk, whether it's
// safe to write, and its debounce state.
type projectEntry struct {
	proj store.Project
	// path is the project's Markdown file, fixed at load or creation
	// time. Renaming a project never moves its file — the id in front
	// matter, not the filename, is what identifies it across restarts.
	path string
	// slug is path's basename without ".md", tracked separately so
	// CreateProject can pick a fresh unique slug without reparsing every
	// known path.
	slug string
	// readOnly is set at load when the file wasn't fully understood — a
	// parse warning, or a schema newer than mdfile.CurrentSchema. Every
	// mutating method rejects with store.ErrReadOnly while this is set.
	readOnly bool
	// lastWritten is the exact bytes last known to be on disk for this
	// project: the raw bytes read at load, or whatever flush most
	// recently wrote. A flush whose freshly formatted bytes equal this
	// is skipped entirely.
	lastWritten []byte
	// dirty is true from the first unsaved mutation until the next
	// successful (or abandoned) flush.
	dirty bool
	// idleTimer fires Debounce after the most recent mutation; every
	// further mutation resets it. maxTimer fires MaxDelay after the
	// first mutation in the current dirty streak and is never reset,
	// capping how long a continuously-edited project can go unsaved.
	idleTimer *time.Timer
	maxTimer  *time.Timer
}

// subEntry is one Subscribe registration. id makes Subscribe's returned
// unsubscribe func able to find and remove exactly this entry — funcs
// aren't comparable in Go, so a slice of these stands in for a set.
type subEntry struct {
	id int
	fn func(store.Event)
}
