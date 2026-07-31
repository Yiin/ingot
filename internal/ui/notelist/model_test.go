package notelist

import (
	"slices"
	"testing"
)

var testSections = []Section{
	{ID: "todo", Title: "To do"},
	{ID: "done", Title: "Done"},
}

func TestNewModelStartsWithOnePlaceholderPerSection(t *testing.T) {
	m := NewModel(testSections)
	if got := m.Len(); got != 2 {
		t.Fatalf("NewModel(2 empty sections).Len() = %d, want 2 (one placeholder each)", got)
	}
	for _, it := range []*Item{m.At(0), m.At(1)} {
		if !it.IsPlaceholder() {
			t.Errorf("At(...) = %+v, want a placeholder", it)
		}
	}
	if m.gl.Len() != m.Len() {
		t.Errorf("gioutil model Len() = %d, want %d (mirrors the base slice)", m.gl.Len(), m.Len())
	}
}

func TestPlaceholderInvariant(t *testing.T) {
	m := NewModel(testSections)

	first := NewItem("n1", "todo", "first note", false)
	m.Append(first)

	// The "todo" placeholder must be gone; "done" keeps its own.
	var placeholders, notes int
	for i := 0; i < m.Len(); i++ {
		it := m.At(i)
		if it.IsPlaceholder() {
			placeholders++
			if it.SectionID != "done" {
				t.Errorf("unexpected placeholder in section %q after adding a todo note", it.SectionID)
			}
		} else {
			notes++
		}
	}
	if placeholders != 1 || notes != 1 {
		t.Fatalf("after one Append: placeholders=%d notes=%d, want 1 and 1", placeholders, notes)
	}

	// Removing the only real note restores its section's placeholder.
	m.RemoveAt(m.IndexOf(first))
	placeholders = 0
	for i := 0; i < m.Len(); i++ {
		if m.At(i).IsPlaceholder() {
			placeholders++
		}
	}
	if placeholders != 2 {
		t.Errorf("after removing the only note: placeholders=%d, want 2 (both sections empty again)", placeholders)
	}
	if m.gl.Len() != m.Len() {
		t.Errorf("gioutil model Len() = %d, want %d", m.gl.Len(), m.Len())
	}
}

func TestModelAppendInsertRemoveMove(t *testing.T) {
	m := NewModel(testSections)

	a := NewItem("a", "todo", "a", false)
	b := NewItem("b", "todo", "b", false)
	c := NewItem("c", "todo", "c", false)
	m.Append(a)
	m.Append(b)
	m.Append(c)

	if got := itemIDs(m.Items()); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("after 3 Appends, Items() = %v, want [a b c]", got)
	}

	d := NewItem("d", "todo", "d", false)
	m.InsertAt(m.IndexOf(a), d)
	if got := itemIDs(m.Items()); !slices.Equal(got, []string{"d", "a", "b", "c"}) {
		t.Fatalf("InsertAt(0, d) -> Items() = %v, want [d a b c]", got)
	}

	m.RemoveAt(m.IndexOf(b))
	if got := itemIDs(m.Items()); !slices.Equal(got, []string{"d", "a", "c"}) {
		t.Fatalf("RemoveAt(b) -> Items() = %v, want [d a c]", got)
	}

	m.Move(m.IndexOf(c), m.IndexOf(d))
	if got := itemIDs(m.Items()); !slices.Equal(got, []string{"c", "d", "a"}) {
		t.Fatalf("Move(c, to front) -> Items() = %v, want [c d a]", got)
	}

	if m.gl.Len() != m.Len() {
		t.Errorf("gioutil model Len() = %d, want %d (mirrors the base slice) after Insert/Remove/Move", m.gl.Len(), m.Len())
	}
}

// TestModelMoveUsesFinalIndexSemantics covers the downward-move
// direction: to is the item's own final base index (the same index
// space At/IndexOf use), not a pre-removal insertion gap. Under the gap
// convention, Move(i, i+1) is a no-op; under final-index semantics
// (what Move documents and this package uses) it is an adjacent swap.
func TestModelMoveUsesFinalIndexSemantics(t *testing.T) {
	m := NewModel([]Section{{ID: "s"}})
	a := NewItem("a", "s", "a", false)
	b := NewItem("b", "s", "b", false)
	c := NewItem("c", "s", "c", false)
	m.Append(a)
	m.Append(b)
	m.Append(c)

	m.Move(0, 1)
	if got := itemIDs(m.Items()); !slices.Equal(got, []string{"b", "a", "c"}) {
		t.Fatalf("Move(0,1) -> Items() = %v, want [b a c]", got)
	}

	m.Move(m.IndexOf(b), 2)
	if got := itemIDs(m.Items()); !slices.Equal(got, []string{"a", "c", "b"}) {
		t.Fatalf("Move(to end) -> Items() = %v, want [a c b]", got)
	}
	if m.gl.Len() != m.Len() {
		t.Errorf("gioutil model Len() = %d, want %d", m.gl.Len(), m.Len())
	}
}

func TestModelAppendAll(t *testing.T) {
	m := NewModel(testSections)
	items := make([]*Item, 0, 5000)
	for i := 0; i < 5000; i++ {
		items = append(items, NewItem("n", "todo", "body", false))
	}
	m.AppendAll(items)

	if got := len(m.Items()); got != 5000 {
		t.Fatalf("AppendAll(5000).Items() len = %d, want 5000", got)
	}
	if m.gl.Len() != m.Len() {
		t.Errorf("gioutil model Len() = %d, want %d", m.gl.Len(), m.Len())
	}
}

func TestModelReset(t *testing.T) {
	m := NewModel(testSections)
	m.Append(NewItem("a", "todo", "a", false))

	fresh := []*Item{
		NewItem("x", "todo", "x", false),
		NewItem("y", "done", "y", true),
	}
	m.Reset(fresh)

	if got := itemIDs(m.Items()); !slices.Equal(got, []string{"x", "y"}) {
		t.Fatalf("Reset(...) -> Items() = %v, want [x y]", got)
	}
	// Both sections are non-empty now, so no placeholders should remain.
	for i := 0; i < m.Len(); i++ {
		if m.At(i).IsPlaceholder() {
			t.Errorf("Reset with a note in every section left a placeholder at base index %d", i)
		}
	}
	if m.gl.Len() != m.Len() {
		t.Errorf("gioutil model Len() = %d, want %d", m.gl.Len(), m.Len())
	}
}

func TestSetSectionsRebuildsPlaceholders(t *testing.T) {
	m := NewModel(testSections)
	m.SetSections([]Section{{ID: "a"}, {ID: "b"}, {ID: "c"}})

	if got := m.Len(); got != 3 {
		t.Fatalf("SetSections(3 sections).Len() = %d, want 3", got)
	}
	for i := 0; i < m.Len(); i++ {
		if !m.At(i).IsPlaceholder() {
			t.Errorf("At(%d) is not a placeholder after SetSections declared it fresh", i)
		}
	}
}

func TestViewPositionMatchesReferenceSort(t *testing.T) {
	m := NewModel(testSections)
	items := []*Item{
		NewItem("a", "done", "a", true),
		NewItem("b", "todo", "b", false),
		NewItem("c", "todo", "c", false),
	}
	for _, it := range items {
		m.Append(it)
	}

	all := make([]*Item, m.Len())
	for i := range all {
		all[i] = m.At(i)
	}
	ref := append([]*Item(nil), all...)
	slices.SortStableFunc(ref, m.compareOrder)

	for wantPos, it := range ref {
		if got := m.ViewPosition(it); got != wantPos {
			t.Errorf("ViewPosition(%v) = %d, want %d (reference sort position)", it, got, wantPos)
		}
	}
}

func TestSectionCountIgnoresPlaceholder(t *testing.T) {
	m := NewModel(testSections)
	if got := m.counts["todo"]; got != 0 {
		t.Errorf("counts[todo] on an empty section = %d, want 0", got)
	}
	m.Append(NewItem("a", "todo", "a", false))
	if got := m.counts["todo"]; got != 1 {
		t.Errorf("counts[todo] after one real note = %d, want 1 (placeholder never counted)", got)
	}
}

func itemIDs(items []*Item) []string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}
