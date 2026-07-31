package store

import "context"

// stub is a minimal Store implementation that exists solely for the
// compile-time assertion below: if Store's method set ever drifts from
// what stub implements, this package fails to build with a direct
// mismatch instead of the drift surfacing later, in fsstore.
type stub struct{}

func (*stub) Projects() []ProjectRef                          { return nil }
func (*stub) Project(ProjectID) (Project, error)              { return Project{}, nil }
func (*stub) Active() ProjectID                               { return "" }
func (*stub) Note(NoteID) (Note, error)                       { return Note{}, nil }
func (*stub) SetActive(ProjectID) error                       { return nil }
func (*stub) AddNote(SectionID, string) (NoteID, error)       { return "", nil }
func (*stub) AppendToDefault(string, Origin) (NoteID, error)  { return "", nil }
func (*stub) SetNoteBody(NoteID, string) error                { return nil }
func (*stub) SetNoteDone(NoteID, bool) error                  { return nil }
func (*stub) DeleteNotes([]NoteID) error                      { return nil }
func (*stub) MoveNotes([]NoteID, SectionID) error             { return nil }
func (*stub) MergeNotes([]NoteID) (NoteID, error)             { return "", nil }
func (*stub) ClearDone() error                                { return nil }
func (*stub) AddSection(ProjectID, string) (SectionID, error) { return "", nil }
func (*stub) RenameSection(SectionID, string) error           { return nil }
func (*stub) DeleteSection(SectionID) error                   { return nil }
func (*stub) MoveSection(SectionID, int) error                { return nil }
func (*stub) CreateProject(string) (ProjectID, error)         { return "", nil }
func (*stub) RenameProject(ProjectID, string) error           { return nil }
func (*stub) DeleteProject(ProjectID) error                   { return nil }
func (*stub) Search(string, Scope) ([]Hit, error)             { return nil, nil }
func (*stub) CanUndo() (string, bool)                         { return "", false }
func (*stub) Undo() error                                     { return nil }
func (*stub) Subscribe(func(Event)) func()                    { return func() {} }
func (*stub) Flush(context.Context) error                     { return nil }
func (*stub) Close() error                                    { return nil }

// var _ Store proves, at compile time, that a nil *stub satisfies Store
// without ever constructing or calling one.
var _ Store = (*stub)(nil)
