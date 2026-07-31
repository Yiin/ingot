package store

import "context"

// Store is Ingot's in-memory note/project model plus persistence. See
// the package doc comment for the threading rule: it is not
// goroutine-safe and must be called only from the goroutine that
// constructed it.
//
// Reads return deep copies: a caller may retain or mutate what it gets
// back without corrupting the Store's own state.
type Store interface {
	// Projects lists every known project as a lightweight summary.
	Projects() []ProjectRef
	// Project returns the full project for id, or ErrNotFound.
	Project(id ProjectID) (Project, error)
	// Active returns the currently active project's id.
	Active() ProjectID
	// Note returns a single note by id, or ErrNotFound.
	Note(id NoteID) (Note, error)

	// SetActive changes the active project, firing ActiveProjectChanged.
	SetActive(id ProjectID) error

	// AddNote appends a SourceTyped note with body to section, firing
	// NotesSpliced. Returns ErrEmptyBody for a blank body and
	// ErrNotFound for an unknown section.
	AddNote(section SectionID, body string) (NoteID, error)
	// AppendToDefault appends a SourceCaptured note to the default
	// capture location, tagging it with origin's App and URI. Used by
	// the double-Shift-tap chord flow, independent of whatever section
	// is currently shown.
	AppendToDefault(body string, origin Origin) (NoteID, error)
	// SetNoteBody replaces a note's body text, firing NoteUpdated.
	// Returns ErrEmptyBody for a blank body.
	SetNoteBody(id NoteID, body string) error
	// SetNoteDone sets a note's done state and DoneAt, firing
	// NoteUpdated.
	SetNoteDone(id NoteID, done bool) error
	// DeleteNotes removes the given notes, firing one NotesSpliced per
	// contiguous run removed.
	DeleteNotes(ids []NoteID) error
	// MoveNotes relocates the given notes to the end of toSection —
	// which may belong to a different project than they currently do —
	// firing a NotesSpliced removal at the source and an insertion at
	// the destination.
	MoveNotes(ids []NoteID, toSection SectionID) error
	// MergeNotes combines two or more notes, in document order —
	// independent of the order ids are given in — into one new
	// SourceMerged note at the position of the document-order-first
	// input, and removes the rest. Returns ErrTooFewNotes for fewer
	// than two ids.
	MergeNotes(ids []NoteID) (NoteID, error)
	// ClearDone removes every done note from the active project.
	ClearDone() error

	// AddSection appends a new section titled title to project, firing
	// SectionsChanged. Returns ErrDuplicateSection if the title
	// collides.
	AddSection(project ProjectID, title string) (SectionID, error)
	// RenameSection renames a section, firing SectionsChanged. Returns
	// ErrDuplicateSection if the title collides with a sibling.
	RenameSection(id SectionID, title string) error
	// DeleteSection removes a section and every note in it, firing
	// SectionsChanged. Returns ErrLastSection if it is its project's
	// only remaining section.
	DeleteSection(id SectionID) error
	// MoveSection reorders a section to index within its project,
	// firing SectionsChanged.
	MoveSection(id SectionID, index int) error

	// CreateProject creates a new project titled title, firing
	// ProjectListChanged. Returns ErrNameTaken if the title collides.
	CreateProject(title string) (ProjectID, error)
	// RenameProject renames a project, firing ProjectListChanged.
	// Returns ErrNameTaken if the title collides with another project.
	RenameProject(id ProjectID, title string) error
	// DeleteProject deletes a project and its file, firing
	// ProjectListChanged.
	DeleteProject(id ProjectID) error

	// Search finds notes whose body matches query within scope,
	// ordered by relevance.
	Search(query string, scope Scope) ([]Hit, error)

	// CanUndo reports whether Undo has anything to reverse. Ingot keeps
	// a single level of undo, not a stack.
	CanUndo() bool
	// Undo reverses the most recent mutation, or is a no-op if CanUndo
	// is false.
	Undo() error

	// Subscribe registers fn to receive every future Event
	// synchronously, on the calling goroutine, until the returned
	// unsubscribe func is called.
	Subscribe(fn func(Event)) (unsubscribe func())

	// Flush blocks until every debounced save has been written to
	// disk, or ctx is done.
	Flush(ctx context.Context) error
	// Close flushes and releases any resources (file watchers, open
	// handles). The Store must not be used afterward.
	Close() error
}
