package menus

// Handlers is the app-level command and state surface every menu action,
// accelerator, and menu-build call goes through. Register wires each
// command method to exactly one action; BuildContext and BuildOverflow's
// callers read the query methods to assemble ContextInfo and the project
// list immediately before showing a menu. Nothing in this package touches
// note or project data directly.
type Handlers interface {
	// Commands.

	Copy()
	CopyAsList()
	MarkDone()
	Expand()
	Edit()
	EditNewWindow()
	Merge()
	// MoveTo receives one of "section:<id>" or "project:<id>" — see the
	// package doc.
	MoveTo(target string)
	// NewSection is called once the user commits a title typed into the
	// Move to submenu's inline "New Section..." entry.
	NewSection(projectID, title string)
	SetProject(id string)
	SetKeepOnTop(on bool)
	ClearDone()
	Shortcuts()
	Close()

	// Queries, read fresh before every menu build.

	// RowIsTruncated reports whether the row at idx is showing an
	// ellipsis, per widget.Label.IsTruncated — only meaningful once the
	// row has been allocated.
	RowIsTruncated(idx int) bool
	RowIsDone(idx int) bool
	RowIsExpanded(idx int) bool
	// SelectionCount is the number of notes in the current selection;
	// Merge Notes needs at least two.
	SelectionCount() int

	CurrentProjectID() string
	CurrentSectionID() string
	// Sections lists the current project's sections in display order.
	Sections() []Section
	// Projects lists every project in display order, including the
	// current one — BuildOverflow's Ctrl+1..9 accelerators number this
	// same order.
	Projects() []Project
	KeepOnTop() bool
}
