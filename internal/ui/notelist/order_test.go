package notelist

import "testing"

// barePool builds a Model with just enough state for rankOf/compareSection
// /compareOrder to run, without going through NewModel — so these tests
// never touch the cgo-backed gioutil list at all.
func barePool(sections []Section) *Model {
	m := &Model{sections: sections, rank: make(map[string]int, len(sections))}
	for i, s := range sections {
		m.rank[s.ID] = i
	}
	return m
}

func TestCompareSectionReturnsRankDifferenceNotStringCompare(t *testing.T) {
	// Declared out of alphabetical order: "z-first" ranks before
	// "a-second". A string compare would reverse this; a rank
	// difference must not.
	m := barePool([]Section{{ID: "z-first"}, {ID: "a-second"}})

	a := &Item{SectionID: "z-first"}
	b := &Item{SectionID: "a-second"}

	if got := m.compareSection(a, b); got >= 0 {
		t.Errorf("compareSection(z-first, a-second) = %d, want < 0 (declared order, not alphabetical)", got)
	}
	if got := m.compareSection(b, a); got <= 0 {
		t.Errorf("compareSection(a-second, z-first) = %d, want > 0", got)
	}
}

func TestCompareSectionIsZeroWithinSection(t *testing.T) {
	m := barePool([]Section{{ID: "s1"}, {ID: "s2"}})
	a := &Item{SectionID: "s1", seq: 0}
	b := &Item{SectionID: "s1", seq: 5}

	if got := m.compareSection(a, b); got != 0 {
		t.Errorf("compareSection within the same section = %d, want 0 (a section is a maximal run of equal items to GtkSortListModel)", got)
	}
}

func TestCompareOrderIsTotal(t *testing.T) {
	m := barePool([]Section{{ID: "s1"}, {ID: "s2"}})
	a := &Item{SectionID: "s1", seq: 3}
	b := &Item{SectionID: "s1", seq: 7}

	if got := m.compareOrder(a, b); got >= 0 {
		t.Errorf("compareOrder(seq 3, seq 7) = %d, want < 0", got)
	}
	if got := m.compareOrder(b, a); got <= 0 {
		t.Errorf("compareOrder(seq 7, seq 3) = %d, want > 0", got)
	}
	if got := m.compareOrder(a, a); got != 0 {
		t.Errorf("compareOrder(a, a) = %d, want 0", got)
	}

	// Cross-section: the section rank must dominate the seq tiebreak.
	c := &Item{SectionID: "s2", seq: 0}
	if got := m.compareOrder(a, c); got >= 0 {
		t.Errorf("compareOrder(s1/seq3, s2/seq0) = %d, want < 0 (section rank dominates seq)", got)
	}
}

func TestUnknownSectionSortsLast(t *testing.T) {
	m := barePool([]Section{{ID: "s1"}, {ID: "s2"}})
	known := &Item{SectionID: "s2"}
	unknown := &Item{SectionID: "does-not-exist"}

	if got := m.compareSection(known, unknown); got >= 0 {
		t.Errorf("compareSection(known, unknown) = %d, want < 0 (unknown sorts last, does not panic)", got)
	}
}
