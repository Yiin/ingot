package store

// Event is anything the Store emits to its subscribers via Subscribe.
// isEvent is unexported so only this package can define event types —
// callers switch over the concrete type; they never implement this
// interface themselves.
type Event interface {
	isEvent()
}

// ProjectListChanged reports that a project was created, deleted,
// renamed, or reordered — anything that changes what Projects returns.
type ProjectListChanged struct{}

// ActiveProjectChanged reports that SetActive moved the active project.
type ActiveProjectChanged struct{}

// SectionsChanged reports that a Project's section list changed shape —
// added, deleted, renamed, or reordered — without saying which project;
// subscribers re-read via Project.
type SectionsChanged struct{}

// NotesSpliced reports a contiguous insertion or removal of Notes within
// one Section, mapping 1:1 onto gio.ListModel.ItemsChanged(position,
// removed, added): Index is position, Removed and Added are counts.
// Anything that changes which notes exist or their order — add, delete,
// move, merge — fires this instead of NoteUpdated, so the list model can
// splice instead of reloading.
type NotesSpliced struct {
	Project int
	Section int
	Index   int
	Removed int
	Added   int
}

// NoteUpdated reports an in-place change to one Note — body text or done
// state — that doesn't change its position, so the list model can
// re-bind exactly one row instead of re-running the whole model. Project
// and Section are indices as in NotesSpliced; Index locates the Note
// within that Section, and ID is the updated Note's id for subscribers
// that track notes by id rather than position.
type NoteUpdated struct {
	Project int
	Section int
	Index   int
	ID      NoteID
}

// ProjectReloaded reports that a Project's file changed outside the
// Store (a hand edit, or another process) and the Store reloaded it
// wholesale; subscribers should refetch via Project rather than trust
// any previously cached position.
type ProjectReloaded struct{}

// ConflictResolved reports that a save collided with an on-disk change
// made outside the Store, and the Store's version was written to
// SavedTo instead of overwriting the newer file. ID is the affected
// project.
type ConflictResolved struct {
	ID      ProjectID
	SavedTo string
}

// SaveFailed reports that a debounced or explicit save failed and the
// in-memory state may now be ahead of disk. Subscribers should surface
// this to the user; the Store keeps retrying via its usual save path.
type SaveFailed struct{}

// ProjectReadOnly reports that a Project failed to load cleanly and is
// now serving reads but rejecting writes with ErrReadOnly.
type ProjectReadOnly struct{}

func (ProjectListChanged) isEvent()   {}
func (ActiveProjectChanged) isEvent() {}
func (SectionsChanged) isEvent()      {}
func (NotesSpliced) isEvent()         {}
func (NoteUpdated) isEvent()          {}
func (ProjectReloaded) isEvent()      {}
func (ConflictResolved) isEvent()     {}
func (SaveFailed) isEvent()           {}
func (ProjectReadOnly) isEvent()      {}
