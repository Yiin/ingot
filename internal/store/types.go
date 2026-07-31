package store

import "time"

// Source records how a Note's body came to exist, so the UI can render
// provenance without guessing.
type Source uint8

const (
	// SourceTyped is a note entered directly into the composer.
	SourceTyped Source = iota
	// SourceCaptured is a note created from a double-Shift-tap text
	// selection capture. App and URI on the Note record where it came
	// from.
	SourceCaptured
	// SourceMerged is a note produced by combining two or more notes via
	// the "Merge Notes" action.
	SourceMerged
	// SourceImported is a note that arrived via a hand edit of the
	// Markdown file on disk, discovered on reload rather than created
	// through the app.
	SourceImported
)

// Note is a single to-do/scratch item: a line, or an indented
// continuation block, in a Section's Markdown file.
type Note struct {
	ID      NoteID
	Body    string
	Done    bool
	DoneAt  time.Time
	Created time.Time
	Source  Source
	// App and URI identify where a captured note's text came from — the
	// foreground application and, when available, the document or page
	// URI. Both are empty unless Source is SourceCaptured.
	App string
	URI string
}

// Section is a named group of Notes within a Project, rendered as a
// header in the panel's list. Order is significant and is exactly slice
// order: there is no Rank or fractional index, because the persistence
// unit is a whole file rewritten atomically on every change.
type Section struct {
	ID    SectionID
	Title string
	Notes []Note
}

// Project is a named collection of Sections, backed by one Markdown file
// under $XDG_DATA_HOME/ingot/projects/. Schema is the front-matter schema
// version. Extra carries any front-matter fields Ingot doesn't recognize,
// round-tripped verbatim rather than dropped, so a file edited by a
// future version doesn't lose data when opened by this one.
type Project struct {
	ID       ProjectID
	Title    string
	Created  time.Time
	Sections []Section
	Schema   int
	Extra    map[string]string
}

// ProjectRef is a lightweight summary of a Project for views that don't
// need every Note loaded — the project switcher and any "which project"
// picker.
type ProjectRef struct {
	ID    ProjectID
	Title string
	// Path is the absolute path of the project's Markdown file on disk.
	Path string
	// Notes is the total note count across all sections.
	Notes int
	// Done is the count of those notes with Done set.
	Done int
}

// Origin identifies the source of a captured note: the foreground
// application at capture time and, when available, the document or page
// URI. Passed to AppendToDefault and recorded onto the resulting Note's
// App and URI fields.
type Origin struct {
	App string
	URI string
}

// Scope limits a Search to the active project or across every project.
type Scope uint8

const (
	// ScopeActiveProject searches only the project returned by Active.
	ScopeActiveProject Scope = iota
	// ScopeAll searches every project.
	ScopeAll
)

// Hit is one Search match. Project, Section, and Note are indices
// locating the matching Note — Project indexes Store.Projects(), Section
// indexes that project's Sections, and Note indexes that section's
// Notes — indices rather than ids, since order is significant and
// SectionID/NoteID don't survive a reload anyway. Index is the separate
// flat row position within the panel's rendered list, for scrolling
// straight to the match. Ranges gives the byte offsets within the Note's
// Body to highlight, each a [start, end) pair.
type Hit struct {
	Project int
	Section int
	Note    int
	Index   int
	Ranges  [][2]int
}
