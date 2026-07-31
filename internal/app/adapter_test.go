package app

import (
	"testing"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsstore"
	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/paths"
	"github.com/Yiin/ingot/internal/ui/notelist"
)

// testLayout mirrors fsstore's own test scaffolding: an in-memory
// filesystem needs no real directories to exist beforehand, since
// fsstore.New creates Projects and State itself.
func testLayout() paths.Layout {
	return paths.Layout{Projects: "/data/projects", State: "/state", Trash: "/data/trash"}
}

// newTestStore returns a real fsstore-backed Store over an in-memory
// filesystem — exercising the adapter against real Store semantics
// (event shapes, index coordinates) rather than a hand-rolled fake that
// could silently drift from them.
func newTestStore(t *testing.T) store.Store {
	t.Helper()
	st, err := fsstore.New(fsstore.Options{FS: fsx.NewMem(), Paths: testLayout()})
	if err != nil {
		t.Fatalf("fsstore.New: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func itemBodies(items []*notelist.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Body
	}
	return out
}

func TestStoreAdapter_SeedLoadsExistingNotes(t *testing.T) {
	st := newTestStore(t)
	projID, err := st.CreateProject("Notes")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	proj, err := st.Project(projID)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	sec := proj.Sections[0].ID
	if _, err := st.AddNote(sec, "first"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if _, err := st.AddNote(sec, "second"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}

	model := notelist.NewModel(nil)
	a := newStoreAdapter(st, model, nil)
	a.Seed()

	got := itemBodies(model.Items())
	if len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Errorf("Items() = %v, want [first second]", got)
	}
}

func TestStoreAdapter_NotesSplicedInsertsLive(t *testing.T) {
	st := newTestStore(t)
	projID, _ := st.CreateProject("Notes")
	proj, _ := st.Project(projID)
	sec := proj.Sections[0].ID

	model := notelist.NewModel(nil)
	a := newStoreAdapter(st, model, nil)
	a.Seed()
	unsub := st.Subscribe(a.OnEvent)
	defer unsub()

	if _, err := st.AddNote(sec, "hello"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}

	got := itemBodies(model.Items())
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("Items() after AddNote = %v, want [hello]", got)
	}
}

func TestStoreAdapter_NoteUpdatedRefreshesInPlace(t *testing.T) {
	st := newTestStore(t)
	projID, _ := st.CreateProject("Notes")
	proj, _ := st.Project(projID)
	sec := proj.Sections[0].ID
	id, err := st.AddNote(sec, "before")
	if err != nil {
		t.Fatalf("AddNote: %v", err)
	}

	model := notelist.NewModel(nil)
	a := newStoreAdapter(st, model, nil)
	a.Seed()
	unsub := st.Subscribe(a.OnEvent)
	defer unsub()

	before := a.itemForNote(id)
	if before == nil {
		t.Fatalf("itemForNote(%q) = nil after Seed", id)
	}
	born := before.Born

	if err := st.SetNoteBody(id, "after"); err != nil {
		t.Fatalf("SetNoteBody: %v", err)
	}

	items := model.Items()
	if len(items) != 1 || items[0].Body != "after" {
		t.Errorf("Items() after SetNoteBody = %v, want [after]", itemBodies(items))
	}
	// Refresh must reuse the same *Item (Born survives), not rebuild it —
	// see Model.Refresh's doc comment on why that matters for the
	// insert-animation contract.
	if items[0].Born != born {
		t.Errorf("Born changed across an in-place update; Refresh should preserve the same *Item")
	}
}

func TestStoreAdapter_DeleteNotesRemovesLive(t *testing.T) {
	st := newTestStore(t)
	projID, _ := st.CreateProject("Notes")
	proj, _ := st.Project(projID)
	sec := proj.Sections[0].ID
	id, _ := st.AddNote(sec, "doomed")

	model := notelist.NewModel(nil)
	a := newStoreAdapter(st, model, nil)
	a.Seed()
	unsub := st.Subscribe(a.OnEvent)
	defer unsub()

	if err := st.DeleteNotes([]store.NoteID{id}); err != nil {
		t.Fatalf("DeleteNotes: %v", err)
	}
	if got := model.Items(); len(got) != 0 {
		t.Errorf("Items() after DeleteNotes = %v, want empty", itemBodies(got))
	}
	if a.itemForNote(id) != nil {
		t.Errorf("itemForNote(%q) still resolves after delete", id)
	}
}

func TestStoreAdapter_ConflictResolvedNotifies(t *testing.T) {
	st := newTestStore(t)
	st.CreateProject("Notes")

	var messages []string
	model := notelist.NewModel(nil)
	a := newStoreAdapter(st, model, func(msg string) { messages = append(messages, msg) })
	a.Seed()

	a.OnEvent(store.ConflictResolved{SavedTo: "/data/trash/notes.trash.md"})

	if len(messages) != 1 {
		t.Fatalf("messages = %v, want exactly one", messages)
	}
	if want := "Reloaded from disk — your version saved to notes.trash.md"; messages[0] != want {
		t.Errorf("message = %q, want %q", messages[0], want)
	}
}
