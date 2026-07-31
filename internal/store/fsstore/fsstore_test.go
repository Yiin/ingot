package fsstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/paths"
)

// --- test scaffolding -------------------------------------------------

func testLayout() paths.Layout {
	return paths.Layout{
		Projects: "/data/projects",
		State:    "/state",
	}
}

// fakeClock gives tests deterministic, orderable Created/DoneAt
// timestamps without ever touching the real clock.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// seqIDs mints predictable, distinct ids so tests can tell newly-created
// entities apart without depending on crypto/rand output.
func seqIDs() func() string {
	var mu sync.Mutex
	n := 0
	return func() string {
		mu.Lock()
		defer mu.Unlock()
		n++
		return fmt.Sprintf("id-%d", n)
	}
}

// opLog records every fsx operation the MemFS performs, so a test can
// assert exactly how many times a project's file was actually written
// (a completed AtomicWrite always ends in one Rename onto the live
// path) without depending on wall-clock timing beyond "wait long
// enough for the debounce to have fired."
type opLog struct {
	mu  sync.Mutex
	ops []logEntry
}

type logEntry struct{ op, path string }

func (l *opLog) record(op, path string) {
	l.mu.Lock()
	l.ops = append(l.ops, logEntry{op, path})
	l.mu.Unlock()
}

func (l *opLog) count(op, path string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, e := range l.ops {
		if e.op == op && e.path == path {
			n++
		}
	}
	return n
}

// writes reports how many completed AtomicWrite calls have landed on
// path — one per Rename onto it.
func (l *opLog) writes(path string) int { return l.count("Rename", path) }

// countPrefix counts op entries whose path starts with prefix — used to
// tally AtomicWrite attempts against a project's randomly-suffixed temp
// file, where every attempt gets a different exact path.
func (l *opLog) countPrefix(op, prefix string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, e := range l.ops {
		if e.op == op && strings.HasPrefix(e.path, prefix) {
			n++
		}
	}
	return n
}

func newStore(t *testing.T, mem *fsx.MemFS, override func(*Options)) store.Store {
	t.Helper()
	opts := Options{FS: mem, Paths: testLayout()}
	if override != nil {
		override(&opts)
	}
	s, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func mustCreateProject(t *testing.T, s store.Store, title string) store.ProjectID {
	t.Helper()
	id, err := s.CreateProject(title)
	if err != nil {
		t.Fatalf("CreateProject(%q): %v", title, err)
	}
	return id
}

func firstSection(t *testing.T, s store.Store, id store.ProjectID) store.SectionID {
	t.Helper()
	p, err := s.Project(id)
	if err != nil {
		t.Fatalf("Project(%s): %v", id, err)
	}
	if len(p.Sections) == 0 {
		t.Fatalf("project %s has no sections", id)
	}
	return p.Sections[0].ID
}

func mustAddNote(t *testing.T, s store.Store, section store.SectionID, body string) store.NoteID {
	t.Helper()
	id, err := s.AddNote(section, body)
	if err != nil {
		t.Fatalf("AddNote(%q): %v", body, err)
	}
	return id
}

// pathFor returns the on-disk path for a project file named base+".md"
// under the test layout's Projects dir. A project's file is named after
// its title's slug (paths.Slug), not its id — pass the slug (or the
// exact seeded filename's base) here, never the ProjectID.
func pathFor(base string) string {
	return "/data/projects/" + base + ".md"
}

// --- construction & basic reads ---------------------------------------

func TestNewRequiresFS(t *testing.T) {
	if _, err := New(Options{Paths: testLayout()}); err == nil {
		t.Fatal("New with nil FS: got nil error, want one")
	}
}

func TestFreshStoreHasNoProjectsAndNoActive(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	if got := s.Projects(); len(got) != 0 {
		t.Errorf("Projects() = %v, want empty", got)
	}
	if got := s.Active(); got != "" {
		t.Errorf("Active() = %q, want empty", got)
	}
}

// --- invariant 1: a note belongs to exactly one section; a section to
// exactly one project ---------------------------------------------------

func TestNoteBelongsToExactlyOneSection(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)

	pid := mustCreateProject(t, s, "Work")
	secA, err := s.AddSection(pid, "A")
	if err != nil {
		t.Fatalf("AddSection A: %v", err)
	}
	secB, err := s.AddSection(pid, "B")
	if err != nil {
		t.Fatalf("AddSection B: %v", err)
	}

	nid := mustAddNote(t, s, secA, "only in A")

	proj, err := s.Project(pid)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	found := 0
	for _, sec := range proj.Sections {
		for _, n := range sec.Notes {
			if n.ID == nid {
				found++
			}
		}
	}
	if found != 1 {
		t.Errorf("note appears in %d sections, want exactly 1", found)
	}

	// Moving it elsewhere removes it from A entirely, not just adds it
	// to the destination.
	if err := s.MoveNotes([]store.NoteID{nid}, secB); err != nil {
		t.Fatalf("MoveNotes: %v", err)
	}
	proj, _ = s.Project(pid)
	found = 0
	for _, sec := range proj.Sections {
		for _, n := range sec.Notes {
			if n.ID == nid {
				found++
			}
		}
	}
	if found != 1 {
		t.Errorf("after move, note appears in %d sections, want exactly 1", found)
	}
}

// --- invariant 2: order within a section is the slice index, which
// equals the file order --------------------------------------------------

func TestOrderWithinSectionMatchesFileOrder(t *testing.T) {
	mem := fsx.NewMem()
	raw := "---\ningot: 1\nid: abcdefabcdef1234\ntitle: Demo\ncreated: 2026-01-01T00:00:00Z\n---\n\n" +
		"## Todo\n\n- [ ] one\n\n- [ ] two\n\n- [ ] three\n"
	seedFile(t, mem, "demo.md", raw)

	s := newStore(t, mem, nil)
	proj, err := s.Project(store.ProjectID("abcdefabcdef1234"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if len(proj.Sections) != 1 || len(proj.Sections[0].Notes) != 3 {
		t.Fatalf("loaded %+v", proj)
	}
	got := []string{
		proj.Sections[0].Notes[0].Body,
		proj.Sections[0].Notes[1].Body,
		proj.Sections[0].Notes[2].Body,
	}
	want := []string{"one", "two", "three"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Notes[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func seedFile(t *testing.T, mem *fsx.MemFS, name, content string) {
	t.Helper()
	if err := mem.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	f, err := mem.Create("/data/projects/" + name)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// --- invariant 3: a project has at least one section; deleting the
// only one returns ErrLastSection ----------------------------------------

func TestDeleteLastSectionReturnsErrLastSection(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Solo")
	sec := firstSection(t, s, pid)

	if err := s.DeleteSection(sec); err != store.ErrLastSection {
		t.Errorf("DeleteSection(only section) = %v, want ErrLastSection", err)
	}
}

// --- invariant 4: the default target is the last section ---------------

func TestAppendToDefaultTargetsLastSection(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")
	sec2, err := s.AddSection(pid, "Second")
	if err != nil {
		t.Fatalf("AddSection: %v", err)
	}

	if _, err := s.AppendToDefault("capture one", store.Origin{App: "chrome"}); err != nil {
		t.Fatalf("AppendToDefault: %v", err)
	}
	if _, err := s.AppendToDefault("capture two", store.Origin{App: "chrome"}); err != nil {
		t.Fatalf("AppendToDefault: %v", err)
	}

	proj, err := s.Project(pid)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if len(proj.Sections[0].Notes) != 0 {
		t.Errorf("section above the last has %d notes, want 0 (untouched)", len(proj.Sections[0].Notes))
	}
	if proj.Sections[1].ID != sec2 {
		t.Fatalf("second section id mismatch")
	}
	if len(proj.Sections[1].Notes) != 2 {
		t.Fatalf("last section has %d notes, want 2", len(proj.Sections[1].Notes))
	}
	if proj.Sections[1].Notes[0].Body != "capture one" || proj.Sections[1].Notes[1].Body != "capture two" {
		t.Errorf("captures landed out of order: %+v", proj.Sections[1].Notes)
	}
	if proj.Sections[1].Notes[0].Source != store.SourceCaptured {
		t.Errorf("captured note Source = %v, want SourceCaptured", proj.Sections[1].Notes[0].Source)
	}
}

// --- invariant 5: section titles are unique at the API boundary; the
// parser tolerates duplicates from a hand edit -----------------------------

func TestSectionTitleUniquenessAtAPIBoundary(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")

	if _, err := s.AddSection(pid, "Todo"); err != nil {
		t.Fatalf("AddSection: %v", err)
	}
	if _, err := s.AddSection(pid, "Todo"); err != store.ErrDuplicateSection {
		t.Errorf("AddSection duplicate = %v, want ErrDuplicateSection", err)
	}

	other, err := s.AddSection(pid, "Other")
	if err != nil {
		t.Fatalf("AddSection Other: %v", err)
	}
	if err := s.RenameSection(other, "Todo"); err != store.ErrDuplicateSection {
		t.Errorf("RenameSection to duplicate = %v, want ErrDuplicateSection", err)
	}
}

func TestParserToleratesDuplicateSectionTitlesFromDisk(t *testing.T) {
	mem := fsx.NewMem()
	raw := "---\ningot: 1\nid: 1111111111111111\ntitle: Hand Edited\ncreated: 2026-01-01T00:00:00Z\n---\n\n" +
		"## Todo\n\n- [ ] a\n\n## Todo\n\n- [ ] b\n"
	seedFile(t, mem, "hand.md", raw)

	s := newStore(t, mem, nil)
	proj, err := s.Project(store.ProjectID("1111111111111111"))
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if len(proj.Sections) != 2 {
		t.Fatalf("got %d sections, want 2 (parser must not merge or reject duplicates)", len(proj.Sections))
	}
	if proj.Sections[0].Title != "Todo" || proj.Sections[1].Title != "Todo" {
		t.Errorf("titles = %q, %q, want both %q", proj.Sections[0].Title, proj.Sections[1].Title, "Todo")
	}
	if proj.Sections[0].ID == proj.Sections[1].ID {
		t.Errorf("duplicate-titled sections must still have distinct ids")
	}
}

// --- invariant 6: Done == false implies DoneAt is zero ------------------

func TestDoneFalseImpliesDoneAtZero(t *testing.T) {
	mem := fsx.NewMem()
	clock := newFakeClock()
	s := newStore(t, mem, func(o *Options) { o.Now = clock.Now })
	pid := mustCreateProject(t, s, "Demo")
	sec := firstSection(t, s, pid)
	nid := mustAddNote(t, s, sec, "task")

	if err := s.SetNoteDone(nid, true); err != nil {
		t.Fatalf("SetNoteDone(true): %v", err)
	}
	n, err := s.Note(nid)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	if n.DoneAt.IsZero() {
		t.Errorf("DoneAt is zero right after marking done")
	}

	if err := s.SetNoteDone(nid, false); err != nil {
		t.Fatalf("SetNoteDone(false): %v", err)
	}
	n, err = s.Note(nid)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	if !n.DoneAt.IsZero() {
		t.Errorf("DoneAt = %v after un-marking done, want zero", n.DoneAt)
	}
}

// --- invariant 7: ClearDone removes every done note across all
// sections but never removes a section, even one that becomes empty ------

func TestClearDoneNeverRemovesSection(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")
	secA := firstSection(t, s, pid)
	secB, err := s.AddSection(pid, "B")
	if err != nil {
		t.Fatalf("AddSection: %v", err)
	}

	a1 := mustAddNote(t, s, secA, "a1 done")
	mustAddNote(t, s, secA, "a2 stays")
	b1 := mustAddNote(t, s, secB, "b1 done")

	if err := s.SetNoteDone(a1, true); err != nil {
		t.Fatalf("SetNoteDone: %v", err)
	}
	if err := s.SetNoteDone(b1, true); err != nil {
		t.Fatalf("SetNoteDone: %v", err)
	}
	if err := s.SetActive(pid); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	if err := s.ClearDone(); err != nil {
		t.Fatalf("ClearDone: %v", err)
	}

	proj, err := s.Project(pid)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if len(proj.Sections) != 2 {
		t.Fatalf("got %d sections after ClearDone, want 2 (sections must never be removed)", len(proj.Sections))
	}
	if len(proj.Sections[0].Notes) != 1 || proj.Sections[0].Notes[0].Body != "a2 stays" {
		t.Errorf("section A notes = %+v, want just %q", proj.Sections[0].Notes, "a2 stays")
	}
	if len(proj.Sections[1].Notes) != 0 {
		t.Errorf("section B notes = %+v, want empty (but section itself must remain)", proj.Sections[1].Notes)
	}
}

// --- invariant 8: DeleteSection never deletes notes — they move to the
// preceding section, or the following one if it was first -----------------

func TestDeleteSectionRelocatesNotes(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")
	s0 := firstSection(t, s, pid)
	s1, err := s.AddSection(pid, "A")
	if err != nil {
		t.Fatalf("AddSection A: %v", err)
	}
	s2, err := s.AddSection(pid, "B")
	if err != nil {
		t.Fatalf("AddSection B: %v", err)
	}

	mustAddNote(t, s, s1, "a1")
	mustAddNote(t, s, s2, "b1")

	// Deleting a non-first section: its notes append to the end of the
	// preceding section.
	if err := s.DeleteSection(s1); err != nil {
		t.Fatalf("DeleteSection(s1): %v", err)
	}
	proj, _ := s.Project(pid)
	if len(proj.Sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(proj.Sections))
	}
	if proj.Sections[0].ID != s0 || len(proj.Sections[0].Notes) != 1 || proj.Sections[0].Notes[0].Body != "a1" {
		t.Fatalf("preceding section = %+v, want to have absorbed a1", proj.Sections[0])
	}

	// Deleting the now-first section: its notes prepend to the
	// following section, ahead of what was already there.
	if err := s.DeleteSection(s0); err != nil {
		t.Fatalf("DeleteSection(s0): %v", err)
	}
	proj, _ = s.Project(pid)
	if len(proj.Sections) != 1 || proj.Sections[0].ID != s2 {
		t.Fatalf("got %+v, want only s2 left", proj.Sections)
	}
	if len(proj.Sections[0].Notes) != 2 || proj.Sections[0].Notes[0].Body != "a1" || proj.Sections[0].Notes[1].Body != "b1" {
		t.Fatalf("following section notes = %+v, want [a1 b1] (relocated notes prepended)", proj.Sections[0].Notes)
	}

	// Now the project's only section: refuses to delete.
	if err := s.DeleteSection(s2); err != store.ErrLastSection {
		t.Errorf("DeleteSection(only remaining) = %v, want ErrLastSection", err)
	}
}

// --- invariant 9: MergeNotes ---------------------------------------------

func TestMergeNotesDocumentOrderFirst(t *testing.T) {
	mem := fsx.NewMem()
	clock := newFakeClock()
	s := newStore(t, mem, func(o *Options) { o.Now = clock.Now })
	pid := mustCreateProject(t, s, "Demo")
	sec := firstSection(t, s, pid)

	id1 := mustAddNote(t, s, sec, "first")
	clock.Advance(time.Minute)
	id2 := mustAddNote(t, s, sec, "second")
	if err := s.SetNoteDone(id2, true); err != nil {
		t.Fatalf("SetNoteDone: %v", err)
	}
	clock.Advance(time.Minute)
	id3 := mustAddNote(t, s, sec, "third")

	n1, _ := s.Note(id1)

	// Scrambled input order; document order must win for position, body
	// join order, and the earliest Created.
	mergedID, err := s.MergeNotes([]store.NoteID{id3, id1, id2})
	if err != nil {
		t.Fatalf("MergeNotes: %v", err)
	}

	proj, err := s.Project(pid)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if len(proj.Sections[0].Notes) != 1 {
		t.Fatalf("got %d notes after merge, want 1: %+v", len(proj.Sections[0].Notes), proj.Sections[0].Notes)
	}
	merged := proj.Sections[0].Notes[0]
	if merged.ID != mergedID {
		t.Errorf("merged note id = %s, want %s", merged.ID, mergedID)
	}
	if merged.Body != "first\n\nsecond\n\nthird" {
		t.Errorf("merged body = %q, want document-order join", merged.Body)
	}
	if merged.Done {
		t.Errorf("merged Done = true, want false (not every input was done)")
	}
	if !merged.Created.Equal(n1.Created) {
		t.Errorf("merged Created = %v, want earliest input's %v", merged.Created, n1.Created)
	}
	if merged.Source != store.SourceMerged {
		t.Errorf("merged Source = %v, want SourceMerged", merged.Source)
	}
}

func TestMergeNotesDoneOnlyIfEveryInputWasDone(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")
	sec := firstSection(t, s, pid)

	id1 := mustAddNote(t, s, sec, "a")
	id2 := mustAddNote(t, s, sec, "b")
	if err := s.SetNoteDone(id1, true); err != nil {
		t.Fatalf("SetNoteDone: %v", err)
	}
	if err := s.SetNoteDone(id2, true); err != nil {
		t.Fatalf("SetNoteDone: %v", err)
	}

	mergedID, err := s.MergeNotes([]store.NoteID{id1, id2})
	if err != nil {
		t.Fatalf("MergeNotes: %v", err)
	}
	n, err := s.Note(mergedID)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	if !n.Done {
		t.Errorf("merged Done = false, want true (every input was done)")
	}
}

func TestMergeNotesTooFewReturnsError(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")
	sec := firstSection(t, s, pid)
	id1 := mustAddNote(t, s, sec, "only one")

	if _, err := s.MergeNotes([]store.NoteID{id1}); err != store.ErrTooFewNotes {
		t.Errorf("MergeNotes(1 id) = %v, want ErrTooFewNotes", err)
	}
}

// --- invariant 10: MoveNotes preserves relative order and inserts
// contiguously at the target index ----------------------------------------

func TestMoveNotesPreservesDocumentOrder(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")
	src := firstSection(t, s, pid)
	dst, err := s.AddSection(pid, "Dest")
	if err != nil {
		t.Fatalf("AddSection: %v", err)
	}

	n1 := mustAddNote(t, s, src, "n1")
	n2 := mustAddNote(t, s, src, "n2")
	n3 := mustAddNote(t, s, src, "n3")

	// Caller passes them out of document order; n2 is left behind.
	if err := s.MoveNotes([]store.NoteID{n3, n1}, dst); err != nil {
		t.Fatalf("MoveNotes: %v", err)
	}

	proj, err := s.Project(pid)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	var srcSec, dstSec store.Section
	for _, sec := range proj.Sections {
		if sec.ID == src {
			srcSec = sec
		}
		if sec.ID == dst {
			dstSec = sec
		}
	}
	if len(srcSec.Notes) != 1 || srcSec.Notes[0].ID != n2 {
		t.Fatalf("source section = %+v, want just n2 left behind", srcSec.Notes)
	}
	if len(dstSec.Notes) != 2 || dstSec.Notes[0].ID != n1 || dstSec.Notes[1].ID != n3 {
		t.Fatalf("dest section = %+v, want [n1 n3] (document order, contiguous)", dstSec.Notes)
	}
}

func TestMoveNotesAppendsToEndOfDestination(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")
	src := firstSection(t, s, pid)
	dst, err := s.AddSection(pid, "Dest")
	if err != nil {
		t.Fatalf("AddSection: %v", err)
	}
	already := mustAddNote(t, s, dst, "already here")
	moving := mustAddNote(t, s, src, "moving in")

	if err := s.MoveNotes([]store.NoteID{moving}, dst); err != nil {
		t.Fatalf("MoveNotes: %v", err)
	}
	proj, _ := s.Project(pid)
	var dstSec store.Section
	for _, sec := range proj.Sections {
		if sec.ID == dst {
			dstSec = sec
		}
	}
	if len(dstSec.Notes) != 2 || dstSec.Notes[0].ID != already || dstSec.Notes[1].ID != moving {
		t.Fatalf("dest = %+v, want existing note first, moved note appended", dstSec.Notes)
	}
}

// --- invariant 11: a body is never empty ---------------------------------

func TestEmptyBodyRejected(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")
	sec := firstSection(t, s, pid)

	if _, err := s.AddNote(sec, ""); err != store.ErrEmptyBody {
		t.Errorf("AddNote(\"\") = %v, want ErrEmptyBody", err)
	}
	if _, err := s.AddNote(sec, "   \n\n  "); err != store.ErrEmptyBody {
		t.Errorf("AddNote(whitespace) = %v, want ErrEmptyBody", err)
	}

	nid := mustAddNote(t, s, sec, "real body")
	if err := s.SetNoteBody(nid, ""); err != store.ErrEmptyBody {
		t.Errorf("SetNoteBody(\"\") = %v, want ErrEmptyBody", err)
	}
}

// --- invariant 12: body normalization ------------------------------------

func TestBodyNormalization(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")
	sec := firstSection(t, s, pid)

	nid := mustAddNote(t, s, sec, "\r\n\r\nline1\r\nline2\r\n\r\n\r\n")
	n, err := s.Note(nid)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	want := "line1\nline2"
	if n.Body != want {
		t.Errorf("Body = %q, want %q", n.Body, want)
	}
}

// --- invariant 13: every mutation emits exactly one event batch and
// schedules exactly one save ----------------------------------------------

func TestStructuralMutationEmitsEventsAndFlushesImmediately(t *testing.T) {
	mem := fsx.NewMem()
	log := &opLog{}
	mem.SetObserver(log.record)
	s := newStore(t, mem, func(o *Options) { o.NewID = seqIDs() })
	pid := mustCreateProject(t, s, "Demo")
	path := pathFor("demo")
	// CreateProject itself flushes immediately (a project-list change).
	base := log.writes(path)

	var events []store.Event
	unsub := s.Subscribe(func(ev store.Event) { events = append(events, ev) })
	defer unsub()

	events = nil
	if _, err := s.AddSection(pid, "New Section"); err != nil {
		t.Fatalf("AddSection: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events for AddSection, want 1: %+v", len(events), events)
	}
	if _, ok := events[0].(store.SectionsChanged); !ok {
		t.Errorf("event = %T, want SectionsChanged", events[0])
	}
	// AddSection is a "section change": flush is immediate, not
	// debounced — no sleep needed before observing the write.
	if got := log.writes(path) - base; got != 1 {
		t.Errorf("writes after AddSection = %d, want 1 (immediate flush)", got)
	}
}

func TestBodyEditIsDebouncedNotImmediate(t *testing.T) {
	mem := fsx.NewMem()
	log := &opLog{}
	mem.SetObserver(log.record)
	s := newStore(t, mem, func(o *Options) { o.NewID = seqIDs() })
	pid := mustCreateProject(t, s, "Demo")
	path := pathFor("demo")
	// CreateProject itself flushes immediately (a project-list change).
	base := log.writes(path)

	sec := firstSection(t, s, pid)
	if _, err := s.AddNote(sec, "typed body"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if got := log.writes(path); got != base {
		t.Errorf("writes right after AddNote = %d, want unchanged at %d (debounced, not immediate)", got, base)
	}
}

// --- invariant 14: the store never writes a project it did not fully
// understand ----------------------------------------------------------------

func TestSchemaTooNewIsReadOnly(t *testing.T) {
	mem := fsx.NewMem()
	raw := "---\ningot: 99\nid: 2222222222222222\ntitle: Future\ncreated: 2026-01-01T00:00:00Z\n---\n\n## S\n\n- [ ] x\n"
	seedFile(t, mem, "future.md", raw)
	log := &opLog{}
	mem.SetObserver(log.record)

	s := newStore(t, mem, nil)
	path := pathFor("future")

	var events []store.Event
	unsub := s.Subscribe(func(ev store.Event) { events = append(events, ev) })
	defer unsub()

	sec := firstSection(t, s, store.ProjectID("2222222222222222"))
	if _, err := s.AddNote(sec, "should be rejected"); err != store.ErrReadOnly {
		t.Errorf("AddNote on schema-too-new project = %v, want ErrReadOnly", err)
	}
	if !hasEvent(events, store.ProjectReadOnly{}) {
		t.Errorf("events = %+v, want a ProjectReadOnly", events)
	}

	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := log.writes(path); got != 0 {
		t.Errorf("writes to schema-too-new file = %d, want 0", got)
	}
	after, err := mem.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(after) != raw {
		t.Errorf("file bytes changed for a read-only project")
	}
}

func TestUnparsableFileIsReadOnly(t *testing.T) {
	mem := fsx.NewMem()
	// Unterminated front matter produces a parse Warning.
	raw := "---\ningot: 1\nid: 3333333333333333\n\n## S\n- [ ] x\n"
	seedFile(t, mem, "messy.md", raw)
	log := &opLog{}
	mem.SetObserver(log.record)

	s := newStore(t, mem, nil)
	path := pathFor("messy") // no valid front matter id was parsed; id is minted fresh

	var refID store.ProjectID
	for _, ref := range s.Projects() {
		if ref.Path == "/data/projects/messy.md" {
			refID = ref.ID
		}
	}
	if refID == "" {
		t.Fatalf("did not find the loaded project by path")
	}
	sec := firstSection(t, s, refID)

	var events []store.Event
	unsub := s.Subscribe(func(ev store.Event) { events = append(events, ev) })
	defer unsub()

	if _, err := s.AddNote(sec, "should be rejected"); err != store.ErrReadOnly {
		t.Errorf("AddNote on unparsable project = %v, want ErrReadOnly", err)
	}
	if !hasEvent(events, store.ProjectReadOnly{}) {
		t.Errorf("events = %+v, want a ProjectReadOnly", events)
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := log.writes(path); got != 0 {
		t.Errorf("writes to unparsable file = %d, want 0", got)
	}
}

func hasEvent(events []store.Event, want store.Event) bool {
	for _, ev := range events {
		if ev == want {
			return true
		}
	}
	return false
}

// --- acceptance criteria: debounce timing --------------------------------

func TestTenRapidEditsProduceExactlyOneWrite(t *testing.T) {
	mem := fsx.NewMem()
	log := &opLog{}
	mem.SetObserver(log.record)
	s := newStore(t, mem, func(o *Options) {
		o.NewID = seqIDs()
		o.Debounce = 30 * time.Millisecond
		o.MaxDelay = 500 * time.Millisecond
	})
	pid := mustCreateProject(t, s, "Demo")
	path := pathFor("demo")
	sec := firstSection(t, s, pid)
	nid := mustAddNote(t, s, sec, "v0")
	base := log.writes(path)

	for i := 0; i < 10; i++ {
		if err := s.SetNoteBody(nid, fmt.Sprintf("v%d", i+1)); err != nil {
			t.Fatalf("SetNoteBody: %v", err)
		}
	}

	time.Sleep(80 * time.Millisecond)
	if got := log.writes(path) - base; got != 1 {
		t.Errorf("writes after 10 rapid edits = %d, want exactly 1", got)
	}
	n, _ := s.Note(nid)
	if n.Body != "v10" {
		t.Errorf("final body = %q, want %q", n.Body, "v10")
	}
}

func TestContinuousStreamRespectsMaxDelayCap(t *testing.T) {
	mem := fsx.NewMem()
	log := &opLog{}
	mem.SetObserver(log.record)
	s := newStore(t, mem, func(o *Options) {
		o.NewID = seqIDs()
		o.Debounce = 20 * time.Millisecond
		o.MaxDelay = 60 * time.Millisecond
	})
	pid := mustCreateProject(t, s, "Demo")
	path := pathFor("demo")
	sec := firstSection(t, s, pid)
	nid := mustAddNote(t, s, sec, "v0")
	base := log.writes(path)

	deadline := time.Now().Add(150 * time.Millisecond)
	i := 0
	for time.Now().Before(deadline) {
		i++
		if err := s.SetNoteBody(nid, fmt.Sprintf("v%d", i)); err != nil {
			t.Fatalf("SetNoteBody: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	if got := log.writes(path) - base; got < 2 {
		t.Errorf("writes during a %v continuous stream with a %v max delay = %d, want at least 2", 150*time.Millisecond, 60*time.Millisecond, got)
	}
}

func TestDeleteNotesWritesImmediately(t *testing.T) {
	mem := fsx.NewMem()
	log := &opLog{}
	mem.SetObserver(log.record)
	s := newStore(t, mem, func(o *Options) { o.NewID = seqIDs() })
	pid := mustCreateProject(t, s, "Demo")
	path := pathFor("demo")
	sec := firstSection(t, s, pid)
	nid := mustAddNote(t, s, sec, "to delete")
	// Force the add itself to land on disk first, so lastWritten
	// reflects a project that has the note — otherwise deleting it
	// right back out would leave Format's bytes identical to what's
	// already on disk and the "skip an unchanged write" optimization
	// (invariant 14) would mask what this test is checking.
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	base := log.writes(path)

	if err := s.DeleteNotes([]store.NoteID{nid}); err != nil {
		t.Fatalf("DeleteNotes: %v", err)
	}
	if got := log.writes(path) - base; got != 1 {
		t.Errorf("writes right after DeleteNotes = %d, want 1 (immediate, no sleep)", got)
	}
}

func TestSettingBodyToCurrentValueWritesNothing(t *testing.T) {
	mem := fsx.NewMem()
	log := &opLog{}
	mem.SetObserver(log.record)
	s := newStore(t, mem, func(o *Options) { o.NewID = seqIDs() })
	pid := mustCreateProject(t, s, "Demo")
	path := pathFor("demo")
	sec := firstSection(t, s, pid)
	nid := mustAddNote(t, s, sec, "steady")

	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	base := log.writes(path)

	if err := s.SetNoteBody(nid, "steady"); err != nil {
		t.Fatalf("SetNoteBody: %v", err)
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := log.writes(path) - base; got != 0 {
		t.Errorf("writes after setting body to its current value = %d, want 0", got)
	}
}

// --- reload round-trip -----------------------------------------------------

func TestMutateCloseReopenYieldsIdenticalModel(t *testing.T) {
	mem := fsx.NewMem()
	s1 := newStore(t, mem, func(o *Options) { o.NewID = seqIDs() })

	pid := mustCreateProject(t, s1, "Roundtrip")
	sec, err := s1.AddSection(pid, "Todo")
	if err != nil {
		t.Fatalf("AddSection: %v", err)
	}
	n1 := mustAddNote(t, s1, sec, "keep me")
	n2 := mustAddNote(t, s1, sec, "and me, done")
	if err := s1.SetNoteDone(n2, true); err != nil {
		t.Fatalf("SetNoteDone: %v", err)
	}
	_ = n1

	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2 := newStore(t, mem, func(o *Options) { o.NewID = seqIDs() })
	proj, err := s2.Project(pid)
	if err != nil {
		t.Fatalf("Project after reopen: %v", err)
	}
	if proj.Title != "Roundtrip" {
		t.Errorf("Title = %q, want %q", proj.Title, "Roundtrip")
	}
	// Default lead section (Title=="") is dropped by the formatter when
	// empty, so only the named "Todo" section round-trips.
	var todo *store.Section
	for i := range proj.Sections {
		if proj.Sections[i].Title == "Todo" {
			todo = &proj.Sections[i]
		}
	}
	if todo == nil {
		t.Fatalf("no Todo section after reload: %+v", proj.Sections)
	}
	if len(todo.Notes) != 2 {
		t.Fatalf("got %d notes after reload, want 2: %+v", len(todo.Notes), todo.Notes)
	}
	if todo.Notes[0].Body != "keep me" || todo.Notes[1].Body != "and me, done" {
		t.Errorf("bodies = %+v", todo.Notes)
	}
	if todo.Notes[0].Done {
		t.Errorf("first note Done = true, want false")
	}
	if !todo.Notes[1].Done {
		t.Errorf("second note Done = false, want true")
	}
}

// --- misc error paths & basic accessors ------------------------------------

func TestCreateProjectDuplicateTitleReturnsErrNameTaken(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	if _, err := s.CreateProject("Work"); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := s.CreateProject("Work"); err != store.ErrNameTaken {
		t.Errorf("CreateProject duplicate = %v, want ErrNameTaken", err)
	}
}

func TestAddNoteUnknownSectionReturnsErrNotFound(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	if _, err := s.AddNote(store.SectionID("nope"), "body"); err != store.ErrNotFound {
		t.Errorf("AddNote(unknown section) = %v, want ErrNotFound", err)
	}
}

func TestSetActivePersistsAcrossReopen(t *testing.T) {
	mem := fsx.NewMem()
	s1 := newStore(t, mem, nil)
	p1 := mustCreateProject(t, s1, "First")
	p2 := mustCreateProject(t, s1, "Second")
	if err := s1.SetActive(p2); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_ = p1

	s2 := newStore(t, mem, nil)
	if got := s2.Active(); got != p2 {
		t.Errorf("Active() after reopen = %s, want %s", got, p2)
	}
}

func TestSearchFindsAndScopesCorrectly(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	p1 := mustCreateProject(t, s, "One")
	p2 := mustCreateProject(t, s, "Two")
	sec1 := firstSection(t, s, p1)
	sec2 := firstSection(t, s, p2)
	mustAddNote(t, s, sec1, "buy milk and eggs")
	mustAddNote(t, s, sec2, "buy stamps")

	if err := s.SetActive(p1); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	hits, err := s.Search("buy", store.ScopeActiveProject)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("ScopeActiveProject hits = %d, want 1: %+v", len(hits), hits)
	}

	hits, err = s.Search("buy", store.ScopeAll)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("ScopeAll hits = %d, want 2: %+v", len(hits), hits)
	}
}

func TestUndoIsANoOpUntilItLands(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	if s.CanUndo() {
		t.Errorf("CanUndo() = true, want false (single-level undo is a separate child)")
	}
	if err := s.Undo(); err != nil {
		t.Errorf("Undo() = %v, want nil", err)
	}
}

func TestRenameProject(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	p1 := mustCreateProject(t, s, "Old Name")
	p2 := mustCreateProject(t, s, "Taken")

	var events []store.Event
	unsub := s.Subscribe(func(ev store.Event) { events = append(events, ev) })
	defer unsub()

	if err := s.RenameProject(p1, "New Name"); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1: %+v", len(events), events)
	}
	if _, ok := events[0].(store.ProjectListChanged); !ok {
		t.Errorf("event = %T, want ProjectListChanged", events[0])
	}
	proj, err := s.Project(p1)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if proj.Title != "New Name" {
		t.Errorf("Title = %q, want %q", proj.Title, "New Name")
	}

	if err := s.RenameProject(p1, "Taken"); err != store.ErrNameTaken {
		t.Errorf("RenameProject to a name in use = %v, want ErrNameTaken", err)
	}
	if err := s.RenameProject(store.ProjectID("nope"), "Whatever"); err != store.ErrNotFound {
		t.Errorf("RenameProject(unknown) = %v, want ErrNotFound", err)
	}
	_ = p2
}

func TestDeleteProjectReassignsActiveAndRemovesFile(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	p1 := mustCreateProject(t, s, "First")
	p2 := mustCreateProject(t, s, "Second")
	if err := s.SetActive(p1); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	var events []store.Event
	unsub := s.Subscribe(func(ev store.Event) { events = append(events, ev) })
	defer unsub()

	if err := s.DeleteProject(p1); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if !hasEvent(events, store.ProjectListChanged{}) {
		t.Errorf("events = %+v, want a ProjectListChanged", events)
	}
	if !hasEvent(events, store.ActiveProjectChanged{}) {
		t.Errorf("events = %+v, want an ActiveProjectChanged (deleted project was active)", events)
	}
	if got := s.Active(); got != p2 {
		t.Errorf("Active() after deleting the active project = %s, want the remaining project %s", got, p2)
	}
	if _, err := s.Project(p1); err != store.ErrNotFound {
		t.Errorf("Project(deleted) = %v, want ErrNotFound", err)
	}
	if _, err := mem.ReadFile(pathFor("first")); err == nil {
		t.Errorf("deleted project's file still exists on disk")
	}

	if err := s.DeleteProject(store.ProjectID("nope")); err != store.ErrNotFound {
		t.Errorf("DeleteProject(unknown) = %v, want ErrNotFound", err)
	}
}

func TestMoveSectionReordersAndClampsIndex(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")
	s0 := firstSection(t, s, pid)
	s1, err := s.AddSection(pid, "A")
	if err != nil {
		t.Fatalf("AddSection A: %v", err)
	}
	s2, err := s.AddSection(pid, "B")
	if err != nil {
		t.Fatalf("AddSection B: %v", err)
	}
	// order: [s0, s1, s2]

	if err := s.MoveSection(s0, 2); err != nil {
		t.Fatalf("MoveSection: %v", err)
	}
	proj, _ := s.Project(pid)
	gotOrder := []store.SectionID{proj.Sections[0].ID, proj.Sections[1].ID, proj.Sections[2].ID}
	want := []store.SectionID{s1, s2, s0}
	for i := range want {
		if gotOrder[i] != want[i] {
			t.Fatalf("order = %v, want %v", gotOrder, want)
		}
	}

	// order is now [s1, s2, s0]; move s2 to a negative index, clamps to 0.
	if err := s.MoveSection(s2, -5); err != nil {
		t.Fatalf("MoveSection: %v", err)
	}
	proj, _ = s.Project(pid)
	if proj.Sections[0].ID != s2 {
		t.Errorf("after moving to a negative index, first section = %s, want %s (clamped to 0)", proj.Sections[0].ID, s2)
	}

	// order is now [s2, s1, s0]; move s1 past the end, clamps to last.
	if err := s.MoveSection(s1, 99); err != nil {
		t.Fatalf("MoveSection: %v", err)
	}
	proj, _ = s.Project(pid)
	if proj.Sections[len(proj.Sections)-1].ID != s1 {
		t.Errorf("after moving past the end, last section = %s, want %s (clamped)", proj.Sections[len(proj.Sections)-1].ID, s1)
	}

	if err := s.MoveSection(store.SectionID("nope"), 0); err != store.ErrNotFound {
		t.Errorf("MoveSection(unknown) = %v, want ErrNotFound", err)
	}
}

// --- regression: non-contiguous removals must emit splices a
// subscriber can apply sequentially against its own copy of the list ---

func TestNonContiguousDeleteEmitsSequentiallyApplicableSplices(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")
	sec := firstSection(t, s, pid)

	bodies := []string{"n0", "n1", "n2", "n3", "n4"}
	ids := make([]store.NoteID, len(bodies))
	for i, b := range bodies {
		ids[i] = mustAddNote(t, s, sec, b)
	}

	var events []store.Event
	unsub := s.Subscribe(func(ev store.Event) { events = append(events, ev) })
	defer unsub()

	// Two separate single-note runs in the same section.
	if err := s.DeleteNotes([]store.NoteID{ids[1], ids[3]}); err != nil {
		t.Fatalf("DeleteNotes: %v", err)
	}

	// NotesSpliced maps 1:1 onto gio.ListModel.ItemsChanged (see
	// events.go): a subscriber applies each event sequentially against
	// its own copy of the list. Mirror that here and check it lands on
	// the actual remaining notes.
	mirror := append([]string(nil), bodies...)
	for _, ev := range events {
		sp, ok := ev.(store.NotesSpliced)
		if !ok {
			continue
		}
		if sp.Index < 0 || sp.Index+sp.Removed > len(mirror) {
			t.Fatalf("NotesSpliced{Index:%d,Removed:%d} out of range for a %d-element mirror — an earlier event must have been applied out of order", sp.Index, sp.Removed, len(mirror))
		}
		mirror = append(mirror[:sp.Index], mirror[sp.Index+sp.Removed:]...)
	}

	proj, err := s.Project(pid)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	var want []string
	for _, n := range proj.Sections[0].Notes {
		want = append(want, n.Body)
	}

	if len(mirror) != len(want) {
		t.Fatalf("mirror = %v (len %d), want %v (len %d)", mirror, len(mirror), want, len(want))
	}
	for i := range want {
		if mirror[i] != want[i] {
			t.Errorf("mirror[%d] = %q, want %q", i, mirror[i], want[i])
		}
	}
}

// --- regression: duplicate ids in a multi-note call must not delete an
// unrelated neighbor -----------------------------------------------------

func TestDuplicateNoteIDsDoNotCauseDataLoss(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	pid := mustCreateProject(t, s, "Demo")
	sec := firstSection(t, s, pid)

	mustAddNote(t, s, sec, "a")
	b := mustAddNote(t, s, sec, "b")
	mustAddNote(t, s, sec, "c")

	if err := s.DeleteNotes([]store.NoteID{b, b}); err != nil {
		t.Fatalf("DeleteNotes([b,b]): %v", err)
	}

	proj, err := s.Project(pid)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	var got []string
	for _, n := range proj.Sections[0].Notes {
		got = append(got, n.Body)
	}
	want := []string{"a", "c"}
	if len(got) != len(want) {
		t.Fatalf("notes after DeleteNotes([b,b]) = %v, want %v (b removed once, a and c untouched)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("notes[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// --- regression: two files sharing a front-matter id must not collide ---

func TestDuplicateProjectIDsGetDistinctIdentities(t *testing.T) {
	mem := fsx.NewMem()
	raw1 := "---\ningot: 1\nid: 4444444444444444\ntitle: Copy One\ncreated: 2026-01-01T00:00:00Z\n---\n\n## S\n\n- [ ] x\n"
	raw2 := "---\ningot: 1\nid: 4444444444444444\ntitle: Copy Two\ncreated: 2026-01-01T00:00:00Z\n---\n\n## S\n\n- [ ] y\n"
	seedFile(t, mem, "copy-one.md", raw1)
	seedFile(t, mem, "copy-two.md", raw2)

	s := newStore(t, mem, nil)
	refs := s.Projects()
	if len(refs) != 2 {
		t.Fatalf("got %d projects, want 2 (both files must load, not collide)", len(refs))
	}
	if refs[0].ID == refs[1].ID {
		t.Fatalf("both projects share id %s — a collision was not resolved", refs[0].ID)
	}

	if err := s.DeleteProject(refs[0].ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if got := s.Projects(); len(got) != 1 {
		t.Fatalf("got %d projects after delete, want 1: %+v", len(got), got)
	}
}

// --- regression: a write-failure retry timer must not keep firing, or
// keep writing, after Close ------------------------------------------------

func TestCloseStopsRetryTimerAfterWriteFailure(t *testing.T) {
	mem := fsx.NewMem()
	log := &opLog{}
	mem.SetObserver(log.record)
	s := newStore(t, mem, func(o *Options) {
		o.NewID = seqIDs()
		o.Debounce = 15 * time.Millisecond
		o.MaxDelay = 500 * time.Millisecond
	})
	pid := mustCreateProject(t, s, "Demo")

	// Every future temp-file create for this project's file fails, so
	// its debounced flush — and its own failure-triggered retry — never
	// succeeds.
	mem.FailOn("Create", "/data/projects/.demo.md.tmp-*", errors.New("disk full"))

	sec := firstSection(t, s, pid)
	if _, err := s.AddNote(sec, "will fail to persist"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}

	// Let the debounced flush fire and fail, and its retry timer fire
	// and fail again at least once, before closing.
	time.Sleep(60 * time.Millisecond)
	attemptsBeforeClose := log.countPrefix("Create", "/data/projects/.demo.md.tmp-")
	if attemptsBeforeClose == 0 {
		t.Fatalf("no write attempts observed before Close — test didn't exercise the failure path")
	}

	if err := s.Close(); err == nil {
		t.Fatalf("Close: want the still-failing flush's error, got nil")
	}
	afterClose := log.countPrefix("Create", "/data/projects/.demo.md.tmp-")

	// A retry timer wrongly left armed by Close would fire well within
	// several Debounce/MaxDelay periods.
	time.Sleep(150 * time.Millisecond)
	final := log.countPrefix("Create", "/data/projects/.demo.md.tmp-")
	if final != afterClose {
		t.Errorf("write attempts grew from %d to %d after Close returned — a retry timer kept firing", afterClose, final)
	}
}
