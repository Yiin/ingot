package menus

// fakeHandlers is a recording Handlers for tests: every command call
// appends to calls, and every query reads back the corresponding field
// so a test can assert on exactly what Register/BuildContext wired up.
type fakeHandlers struct {
	calls []string

	truncated map[int]bool
	done      map[int]bool
	expanded  map[int]bool
	selection int

	projectID string
	sectionID string
	sections  []Section
	projects  []Project
	keepOnTop bool

	lastMoveTo       string
	lastNewSection   [2]string
	lastSetProject   string
	lastSetKeepOnTop bool
}

func newFakeHandlers() *fakeHandlers {
	return &fakeHandlers{
		truncated: map[int]bool{},
		done:      map[int]bool{},
		expanded:  map[int]bool{},
	}
}

func (f *fakeHandlers) Copy()          { f.calls = append(f.calls, "copy") }
func (f *fakeHandlers) CopyAsList()    { f.calls = append(f.calls, "copy-as-list") }
func (f *fakeHandlers) MarkDone()      { f.calls = append(f.calls, "mark-done") }
func (f *fakeHandlers) Expand()        { f.calls = append(f.calls, "expand") }
func (f *fakeHandlers) Edit()          { f.calls = append(f.calls, "edit") }
func (f *fakeHandlers) EditNewWindow() { f.calls = append(f.calls, "edit-new-window") }
func (f *fakeHandlers) Merge()         { f.calls = append(f.calls, "merge") }
func (f *fakeHandlers) Shortcuts()     { f.calls = append(f.calls, "shortcuts") }
func (f *fakeHandlers) Close()         { f.calls = append(f.calls, "close") }
func (f *fakeHandlers) ClearDone()     { f.calls = append(f.calls, "clear-done") }

func (f *fakeHandlers) MoveTo(target string) {
	f.calls = append(f.calls, "move-to")
	f.lastMoveTo = target
}

func (f *fakeHandlers) NewSection(projectID, title string) {
	f.calls = append(f.calls, "new-section")
	f.lastNewSection = [2]string{projectID, title}
}

func (f *fakeHandlers) SetProject(id string) {
	f.calls = append(f.calls, "set-project")
	f.lastSetProject = id
}

func (f *fakeHandlers) SetKeepOnTop(on bool) {
	f.calls = append(f.calls, "set-keep-on-top")
	f.lastSetKeepOnTop = on
}

func (f *fakeHandlers) RowIsTruncated(idx int) bool { return f.truncated[idx] }
func (f *fakeHandlers) RowIsDone(idx int) bool      { return f.done[idx] }
func (f *fakeHandlers) RowIsExpanded(idx int) bool  { return f.expanded[idx] }
func (f *fakeHandlers) SelectionCount() int         { return f.selection }

func (f *fakeHandlers) CurrentProjectID() string { return f.projectID }
func (f *fakeHandlers) CurrentSectionID() string { return f.sectionID }
func (f *fakeHandlers) Sections() []Section      { return f.sections }
func (f *fakeHandlers) Projects() []Project      { return f.projects }
func (f *fakeHandlers) KeepOnTop() bool          { return f.keepOnTop }

var _ Handlers = (*fakeHandlers)(nil)
