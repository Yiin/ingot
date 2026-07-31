package search

import (
	"fmt"
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/ui/notelist"
)

// newFixture builds the same section/item shape a live panel would have,
// through notelist.NewModel + notelist.NewItem — both plain Go, no GTK
// widget construction, so (unlike notelist.New/List, which needs a real
// GDK display — see notelist's own integration-tag convention) this runs
// in any sandbox.
func newFixture() ([]notelist.Section, []*notelist.Item, map[string]*notelist.Item) {
	sections := []notelist.Section{
		{ID: "inbox", Title: "Inbox"},
		{ID: "work", Title: "Work"},
		{ID: "ideas", Title: "Ideas"},
	}
	items := map[string]*notelist.Item{
		"a": notelist.NewItem("a", "inbox", "buy milk and eggs", false),
		"b": notelist.NewItem("b", "inbox", "call the dentist", false),
		"c": notelist.NewItem("c", "work", "write the quarterly report", false),
		"d": notelist.NewItem("d", "ideas", "a napkin sketch", false),
	}
	ordered := []*notelist.Item{items["a"], items["b"], items["c"], items["d"]}
	return sections, ordered, items
}

func TestComputeHidesSectionsWithNoMatch(t *testing.T) {
	sections, ordered, _ := newFixture()
	c := New(nil)

	r := c.compute(sections, ordered, nil, "milk")
	if r.showAll {
		t.Fatalf("compute(%q).showAll = true, want false", "milk")
	}
	if r.visibleCount != 1 {
		t.Fatalf("visibleCount = %d, want 1", r.visibleCount)
	}
	if r.visible["inbox"] != true {
		t.Errorf(`visible["inbox"] = false, want true`)
	}
	for _, id := range []string{"work", "ideas"} {
		if r.visible[id] {
			t.Errorf("visible[%q] = true, want false (no matches)", id)
		}
	}
}

func TestComputeEmptyQueryShowsEverythingAndClearsRanges(t *testing.T) {
	sections, ordered, items := newFixture()
	c := New(nil)

	c.compute(sections, ordered, nil, "milk")
	r := c.compute(sections, ordered, nil, "")
	if !r.showAll {
		t.Fatalf("compute(\"\").showAll = false, want true")
	}
	if r.visibleCount != len(ordered) {
		t.Fatalf("visibleCount = %d, want %d", r.visibleCount, len(ordered))
	}
	for id, it := range items {
		if it.Ranges != nil {
			t.Errorf("item %s still carries Ranges %v after compute(\"\")", id, it.Ranges)
		}
	}
}

func TestComputeSetsRangesOnlyOnMatchedItems(t *testing.T) {
	sections, ordered, items := newFixture()
	c := New(nil)
	c.compute(sections, ordered, nil, "dentist")

	if len(items["b"].Ranges) == 0 {
		t.Errorf("matched item b has no Ranges")
	}
	for _, id := range []string{"a", "c", "d"} {
		if items[id].Ranges != nil {
			t.Errorf("non-matched item %s carries Ranges %v", id, items[id].Ranges)
		}
	}
}

func TestComputeTitleMatchShowsEverySectionNote(t *testing.T) {
	sections := []notelist.Section{
		{ID: "inbox", Title: "Inbox"},
		{ID: "work", Title: "Work"},
	}
	a := notelist.NewItem("a", "work", "totally unrelated body", false)
	b := notelist.NewItem("b", "work", "another unrelated body", false)
	ordered := []*notelist.Item{a, b}

	c := New(nil)
	r := c.compute(sections, ordered, nil, "work")
	if r.visibleCount != 2 {
		t.Fatalf("visibleCount = %d, want 2 (title cascade)", r.visibleCount)
	}
	if !r.visible["work"] {
		t.Errorf(`visible["work"] = false, want true`)
	}
	if !r.show[a] || !r.show[b] {
		t.Errorf("show[a]=%v show[b]=%v, want both true", r.show[a], r.show[b])
	}
	if a.Ranges != nil || b.Ranges != nil {
		t.Errorf("title-cascaded notes should carry no highlight Ranges: a=%v b=%v", a.Ranges, b.Ranges)
	}
}

func TestComputeTitleMatchKeepsEmptySectionVisible(t *testing.T) {
	sections := []notelist.Section{
		{ID: "inbox", Title: "Inbox"},
		{ID: "work", Title: "Work"}, // no real notes at all
	}
	a := notelist.NewItem("a", "inbox", "unrelated", false)
	ordered := []*notelist.Item{a}

	c := New(nil)
	r := c.compute(sections, ordered, nil, "work")
	// "work" matches the empty section's own title: 0 real notes, but
	// the section — and so the caller's filter predicate for its
	// placeholder card — must still show.
	if r.visibleCount != 0 {
		t.Fatalf("visibleCount = %d, want 0 (no real notes match)", r.visibleCount)
	}
	if !r.visible["work"] {
		t.Errorf(`visible["work"] = false, want true (title match)`)
	}
}

func TestComputeMovesFocusToFirstMatchWhenFocusedHidden(t *testing.T) {
	sections, ordered, items := newFixture()
	c := New(nil)

	r := c.compute(sections, ordered, items["c"], "milk") // c (work) not visible under "milk"
	if r.focus != items["a"] {
		t.Errorf("focus = %v, want the first (only) match %v", r.focus, items["a"])
	}
}

func TestComputeKeepsFocusWhenFocusedItemStillMatches(t *testing.T) {
	sections, ordered, items := newFixture()
	c := New(nil)

	r := c.compute(sections, ordered, items["a"], "milk")
	if r.focus != nil {
		t.Errorf("focus = %v, want nil (unchanged, a still matches)", r.focus)
	}
}

func TestComputeNoFocusChangeWhenNothingMatches(t *testing.T) {
	sections, ordered, items := newFixture()
	c := New(nil)

	r := c.compute(sections, ordered, items["a"], "xyzzy-no-match")
	if r.focus != nil {
		t.Errorf("focus = %v, want nil (no matches to focus)", r.focus)
	}
}

func TestNormalizedForIsCachedAcrossCallsWithUnchangedBody(t *testing.T) {
	sections, ordered, items := newFixture()
	c := New(nil)

	c.compute(sections, ordered, nil, "milk")
	before, ok := c.cache[items["a"]]
	if !ok {
		t.Fatalf("item a has no cache entry after first compute")
	}
	c.compute(sections, ordered, nil, "dentist")
	after, ok := c.cache[items["a"]]
	if !ok {
		t.Fatalf("item a's cache entry evicted across compute calls")
	}
	if before.norm.Text != after.norm.Text {
		t.Errorf("cached Normalized.Text changed across calls with an unchanged body")
	}
}

func TestEvictStaleDropsRemovedItems(t *testing.T) {
	sections, ordered, items := newFixture()
	c := New(nil)
	c.compute(sections, ordered, nil, "milk")

	remaining := []*notelist.Item{items["b"], items["c"], items["d"]}
	c.evictStale(remaining)

	if _, ok := c.cache[items["a"]]; ok {
		t.Errorf("evictStale did not drop item a, which is no longer in the model")
	}
	if _, ok := c.cache[items["b"]]; !ok {
		t.Errorf("evictStale dropped item b, which is still in the model")
	}
}

// BenchmarkCompute_3000Notes must stay within one frame (~16ms) per the
// child issue's acceptance criteria — mirrors internal/store/fsstore's
// own BenchmarkSearch_3000Notes: a realistic mix (most notes irrelevant
// to the query, each still carrying enough Markdown to exercise the
// per-note parse the cache amortizes away), one untimed compute call to
// warm the cache first, then measuring exactly the steady-state
// per-keystroke cost.
func BenchmarkCompute_3000Notes(b *testing.B) {
	sections := []notelist.Section{{ID: "inbox", Title: "Inbox"}}
	items := make([]*notelist.Item, 3000)
	for i := range items {
		body := fmt.Sprintf("Note %d: use **config-%d** for `service-%d` — see [docs](https://example.com/%d).", i, i, i, i)
		if i%750 == 0 {
			body = fmt.Sprintf("Note %d: use **TOML** as the config format — see the schema docs.", i)
		}
		items[i] = notelist.NewItem(fmt.Sprintf("n%d", i), "inbox", body, false)
	}

	c := New(nil)
	c.compute(sections, items, nil, "toml schema")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.compute(sections, items, nil, "toml schema")
	}
	b.StopTimer()

	if b.Elapsed()/time.Duration(b.N) > 16*time.Millisecond {
		b.Fatalf("average compute = %v, want under 16ms (one frame)", b.Elapsed()/time.Duration(b.N))
	}
}
