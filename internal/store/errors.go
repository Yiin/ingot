package store

import "errors"

// Sentinel errors returned by Store methods. Callers compare with
// errors.Is.
var (
	// ErrNotFound is returned when a Project, Section, or Note id
	// doesn't resolve to anything the Store currently holds.
	ErrNotFound = errors.New("store: not found")

	// ErrEmptyBody is returned by any method that would create or
	// update a Note with a body that is empty after trimming
	// whitespace.
	ErrEmptyBody = errors.New("store: note body is empty")

	// ErrDuplicateSection is returned by AddSection or RenameSection
	// when the requested title already names a Section in the same
	// Project.
	ErrDuplicateSection = errors.New("store: section title already exists in this project")

	// ErrLastSection is returned by DeleteSection when it targets a
	// Project's only remaining Section — a Project must always have at
	// least one place for notes to live.
	ErrLastSection = errors.New("store: cannot delete a project's last section")

	// ErrNameTaken is returned by CreateProject or RenameProject when
	// the requested title collides with an existing project's title.
	ErrNameTaken = errors.New("store: project title already exists")

	// ErrReadOnly is returned by a mutating method on a Project that
	// failed to load cleanly and was opened read-only rather than risk
	// a write that clobbers unparsed content.
	ErrReadOnly = errors.New("store: project is read-only")

	// ErrTooFewNotes is returned by MergeNotes when given fewer than
	// two note ids — there is nothing to merge.
	ErrTooFewNotes = errors.New("store: need at least two notes to merge")
)
