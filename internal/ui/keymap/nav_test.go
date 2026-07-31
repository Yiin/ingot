package keymap

import (
	"slices"
	"testing"
)

// fixtureRows is the acceptance criteria's 6-note, 2-section fixture:
// s1 holds n1..n3, s2 holds n4..n6, in that display order.
var fixtureRows = []Row{
	{ID: "n1", SectionID: "s1"},
	{ID: "n2", SectionID: "s1"},
	{ID: "n3", SectionID: "s1"},
	{ID: "n4", SectionID: "s2"},
	{ID: "n5", SectionID: "s2"},
	{ID: "n6", SectionID: "s2"},
}

// TestNavKeySequence runs the acceptance criteria's ~25-key-sequence
// scenario over the fixture, asserting focus and the selection set
// after every single step — each step's effect depends on the Nav
// state the previous steps left behind, exactly like a real session of
// keyboard and mouse input.
func TestNavKeySequence(t *testing.T) {
	n := NewNav(fixtureRows)

	steps := []struct {
		name         string
		apply        func()
		wantFocus    string
		wantSelected []string
	}{
		{"Down from nothing focused", n.FocusNext, "n1", []string{"n1"}},
		{"Down", n.FocusNext, "n2", []string{"n2"}},
		{"Down", n.FocusNext, "n3", []string{"n3"}},
		{"Down crosses into s2", n.FocusNext, "n4", []string{"n4"}},
		{"Ctrl+Down in the last section is a no-op", n.JumpNextSection, "n4", []string{"n4"}},
		{"Ctrl+Up jumps to the first row of s1", n.JumpPreviousSection, "n1", []string{"n1"}},
		{"Ctrl+Up in the first section is a no-op", n.JumpPreviousSection, "n1", []string{"n1"}},
		{"Down", n.FocusNext, "n2", []string{"n2"}},
		{"Shift+Down extends from the anchor at n2", n.ExtendDown, "n3", []string{"n2", "n3"}},
		{"Shift+Down extends further", n.ExtendDown, "n4", []string{"n2", "n3", "n4"}},
		{"Shift+Up retreats toward the anchor", n.ExtendUp, "n3", []string{"n2", "n3"}},
		{"Shift+Up reaches exactly the anchor", n.ExtendUp, "n2", []string{"n2"}},
		{"Shift+Up crosses past the anchor", n.ExtendUp, "n1", []string{"n1", "n2"}},
		{"Shift+Up at the first row is a no-op", n.ExtendUp, "n1", []string{"n1", "n2"}},
		{"Home collapses the selection", n.FocusFirst, "n1", []string{"n1"}},
		{"End jumps to the last row", n.FocusLast, "n6", []string{"n6"}},
		{"Up moves back one, collapsing", n.FocusPrevious, "n5", []string{"n5"}},
		{"Ctrl+A selects only n5's own section", n.SelectAllInSection, "n5", []string{"n4", "n5", "n6"}},
		{"Escape's clear-selection step empties it, focus stays", n.ClearSelection, "n5", nil},
		{"Ctrl+click n1 toggles it on", func() { n.ToggleClick(0) }, "n1", []string{"n1"}},
		{"Ctrl+click n1 again toggles it off", func() { n.ToggleClick(0) }, "n1", nil},
		{"Ctrl+click n3 toggles it on", func() { n.ToggleClick(2) }, "n3", []string{"n3"}},
		{"Ctrl+click n5 adds it without disturbing n3", func() { n.ToggleClick(4) }, "n5", []string{"n3", "n5"}},
		{"Shift+click n1 ranges from the n5 anchor, replacing the selection", func() { n.RangeClick(0) }, "n1", []string{"n1", "n2", "n3", "n4", "n5"}},
		{"Shift+click n6 re-ranges from the same n5 anchor", func() { n.RangeClick(5) }, "n6", []string{"n5", "n6"}},
	}

	if len(steps) < 20 {
		t.Fatalf("test scaffolding error: only %d steps, want at least ~25 per the acceptance criteria", len(steps))
	}

	for i, s := range steps {
		s.apply()
		if got := n.FocusedID(); got != s.wantFocus {
			t.Fatalf("step %d (%s): FocusedID() = %q, want %q", i+1, s.name, got, s.wantFocus)
		}
		got := n.Selected()
		if !slices.Equal(got, s.wantSelected) {
			t.Fatalf("step %d (%s): Selected() = %v, want %v", i+1, s.name, got, s.wantSelected)
		}
	}
}

func TestNavEmptyIsInert(t *testing.T) {
	n := NewNav(nil)
	n.FocusNext()
	n.FocusPrevious()
	n.FocusFirst()
	n.FocusLast()
	n.ExtendDown()
	n.ExtendUp()
	n.JumpNextSection()
	n.JumpPreviousSection()
	n.SelectAllInSection()
	if got := n.Focus(); got != -1 {
		t.Errorf("Focus() on an empty Nav = %d, want -1", got)
	}
	if got := n.Selected(); len(got) != 0 {
		t.Errorf("Selected() on an empty Nav = %v, want empty", got)
	}
}

func TestNavSetRowsKeepsFocusAndSelectionByID(t *testing.T) {
	n := NewNav(fixtureRows)
	n.RangeClick(1)  // anchor unset -> anchor=1; focus=1 (n2); selects n2 alone? anchor was -1 so set to 1, range(1,1)={n2}
	n.ToggleClick(3) // adds n4, focus/anchor move to 3

	reordered := []Row{
		fixtureRows[5], fixtureRows[3], fixtureRows[1], // n6, n4, n2
	}
	n.SetRows(reordered)

	if got, want := n.FocusedID(), "n4"; got != want {
		t.Errorf("after SetRows, FocusedID() = %q, want %q", got, want)
	}
	if got, want := n.Selected(), []string{"n4", "n2"}; !slices.Equal(got, want) {
		t.Errorf("after SetRows, Selected() = %v, want %v", got, want)
	}
}

func TestNavSetRowsDropsMissingSelection(t *testing.T) {
	n := NewNav(fixtureRows)
	n.ToggleClick(0)
	n.ToggleClick(1)

	n.SetRows([]Row{fixtureRows[1], fixtureRows[2]}) // n1 dropped, n2/n3 kept

	if got, want := n.Selected(), []string{"n2"}; !slices.Equal(got, want) {
		t.Errorf("Selected() = %v, want %v (n1 dropped, its own row gone)", got, want)
	}
}
