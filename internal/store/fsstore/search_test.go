package fsstore

import (
	"fmt"
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsx"
)

func TestSearchIsMarkdownAndDiacriticInsensitive(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	p := mustCreateProject(t, s, "Config")
	sec := firstSection(t, s, p)
	id := mustAddNote(t, s, sec, "Use **TOML as the default declarative format**")
	mustAddNote(t, s, sec, "unrelated café note")

	hits, err := s.Search("toml default", store.ScopeActiveProject)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1: %+v", len(hits), hits)
	}
	if len(hits[0].Ranges) != 2 {
		t.Fatalf("Ranges = %v, want 2 entries", hits[0].Ranges)
	}
	note, err := s.Note(id)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	if got := note.Body[hits[0].Ranges[0][0]:hits[0].Ranges[0][1]]; got != "TOML" {
		t.Errorf("Ranges[0] slices to %q, want %q", got, "TOML")
	}
	if got := note.Body[hits[0].Ranges[1][0]:hits[0].Ranges[1][1]]; got != "default" {
		t.Errorf("Ranges[1] slices to %q, want %q", got, "default")
	}

	hits, err = s.Search("cafe", store.ScopeActiveProject)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("diacritic-insensitive hits = %d, want 1: %+v", len(hits), hits)
	}
}

func TestSearchMarkerOnlyTokenCreatesNoFalseHit(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	p := mustCreateProject(t, s, "P")
	sec := firstSection(t, s, p)
	mustAddNote(t, s, sec, "Use **TOML** now")

	hits, err := s.Search("**", store.ScopeActiveProject)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf(`Search("**") hits = %d, want 0 (stripped Markdown syntax, not content)`, len(hits))
	}
}

func TestSearchReflectsBodyEditImmediately(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	p := mustCreateProject(t, s, "P")
	sec := firstSection(t, s, p)
	id := mustAddNote(t, s, sec, "buy milk")

	if hits, _ := s.Search("bread", store.ScopeActiveProject); len(hits) != 0 {
		t.Fatalf("before edit: hits = %d, want 0", len(hits))
	}

	if err := s.SetNoteBody(id, "buy bread"); err != nil {
		t.Fatalf("SetNoteBody: %v", err)
	}

	hits, err := s.Search("bread", store.ScopeActiveProject)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("after edit: hits = %d, want 1 — stale cache not invalidated?", len(hits))
	}

	if hits, _ := s.Search("milk", store.ScopeActiveProject); len(hits) != 0 {
		t.Fatalf("old body should no longer match: hits = %d, want 0", len(hits))
	}
}

// TestSearchCacheEvictsRemovedNotes guards against searchCache growing
// with every note ever seen in the process's lifetime instead of every
// note currently live: DeleteNotes, MergeNotes' consumed inputs, and
// DeleteProject must all drop their departed NoteIDs from the cache.
func TestSearchCacheEvictsRemovedNotes(t *testing.T) {
	mem := fsx.NewMem()
	st := newStore(t, mem, nil)
	s := st.(*fileStore)
	p := mustCreateProject(t, s, "P")
	sec := firstSection(t, s, p)
	a := mustAddNote(t, s, sec, "alpha")
	b := mustAddNote(t, s, sec, "beta")
	c := mustAddNote(t, s, sec, "gamma")

	if _, err := s.Search("alpha", store.ScopeActiveProject); err != nil {
		t.Fatalf("Search: %v", err)
	}
	for _, id := range []store.NoteID{a, b, c} {
		if _, ok := s.searchCache[id]; !ok {
			t.Fatalf("cache missing entry for %s after warming Search", id)
		}
	}

	if err := s.DeleteNotes([]store.NoteID{a}); err != nil {
		t.Fatalf("DeleteNotes: %v", err)
	}
	if _, ok := s.searchCache[a]; ok {
		t.Error("DeleteNotes left a stale cache entry behind")
	}

	if _, err := s.MergeNotes([]store.NoteID{b, c}); err != nil {
		t.Fatalf("MergeNotes: %v", err)
	}
	if _, ok := s.searchCache[b]; ok {
		t.Error("MergeNotes left a stale cache entry for a consumed input behind")
	}
	if _, ok := s.searchCache[c]; ok {
		t.Error("MergeNotes left a stale cache entry for a consumed input behind")
	}

	if err := s.DeleteProject(p); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if len(s.searchCache) != 0 {
		t.Errorf("searchCache after DeleteProject = %d entries, want 0: %v", len(s.searchCache), s.searchCache)
	}
}

func TestSearchEmptyQueryReturnsNoHits(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, nil)
	p := mustCreateProject(t, s, "P")
	sec := firstSection(t, s, p)
	mustAddNote(t, s, sec, "buy milk")

	hits, err := s.Search("   ", store.ScopeActiveProject)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if hits != nil {
		t.Errorf("Search(whitespace) = %v, want nil", hits)
	}
}

// BenchmarkSearch_3000Notes must stay under 2ms per the child issue's
// acceptance criteria — a plain AND-substring scan over ~600KB of
// cached normalized text, no index. The cache is warmed with one
// untimed Search call first: fsstore's design amortizes the Markdown
// parse across repeated searches against an unchanged body (the
// realistic hot path — typing into the search box without editing
// notes in between), and the benchmark measures exactly that steady
// state rather than the one-time cold-cache cost.
func BenchmarkSearch_3000Notes(b *testing.B) {
	mem := fsx.NewMem()
	opts := Options{FS: mem, Paths: testLayout()}
	s, err := New(opts)
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	pid, err := s.CreateProject("Bench")
	if err != nil {
		b.Fatalf("CreateProject: %v", err)
	}
	proj, err := s.Project(pid)
	if err != nil {
		b.Fatalf("Project: %v", err)
	}
	sec := proj.Sections[0].ID
	// Realistic mix: most notes are irrelevant to the query (the common
	// case — a search term matches a handful of notes, not all of
	// them), each still carrying enough Markdown to exercise the same
	// per-note parse the cache amortizes away.
	for i := 0; i < 3000; i++ {
		body := fmt.Sprintf("Note %d: use **config-%d** for `service-%d` — see [docs](https://example.com/%d).", i, i, i, i)
		if i%750 == 0 {
			body = fmt.Sprintf("Note %d: use **TOML** as the config format — see the schema docs.", i)
		}
		if _, err := s.AddNote(sec, body); err != nil {
			b.Fatalf("AddNote: %v", err)
		}
	}

	if _, err := s.Search("toml schema", store.ScopeActiveProject); err != nil {
		b.Fatalf("warm-up Search: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Search("toml schema", store.ScopeActiveProject); err != nil {
			b.Fatalf("Search: %v", err)
		}
	}
	b.StopTimer()

	if b.Elapsed()/time.Duration(b.N) > 2*time.Millisecond {
		b.Fatalf("average Search = %v, want under 2ms", b.Elapsed()/time.Duration(b.N))
	}
}
