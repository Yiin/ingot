package store

import (
	"crypto/rand"
	"encoding/hex"
)

// ProjectID identifies a Project. It is persisted: stored in the "id"
// front-matter field of the project's Markdown file, so it survives
// restarts and keeps identifying the same project across edits.
type ProjectID string

// SectionID identifies a Section within a Project. It is not persisted —
// the Markdown grammar records only "## " section titles — so it is
// minted fresh by NewID whenever the project loads, and is stable only
// for the lifetime of the in-memory Project it belongs to.
type SectionID string

// NoteID identifies a Note within a Section. Like SectionID, it is not
// persisted: nothing outside one process references a note by id, so ids
// are minted at load. This is what keeps the Markdown file clean and
// hand-editable — nothing about a note's on-disk text ever encodes an id
// nobody asked for.
type NoteID string

// NewID returns a fresh identifier: 16 lowercase hex characters drawn
// from crypto/rand. It backs ProjectID, SectionID, and NoteID alike;
// callers mint one whenever a value needs an id, whether or not that id
// is ever persisted.
func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("store: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}
