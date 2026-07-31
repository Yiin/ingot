package searchtext

import (
	"testing"

	"github.com/Yiin/ingot/internal/store"
)

func sections() []store.Section {
	return []store.Section{
		{
			Title: "Groceries",
			Notes: []store.Note{
				{Body: "buy milk"},
				{Body: "buy eggs"},
			},
		},
		{
			Title: "Ideas",
			Notes: []store.Note{
				{Body: "write a novel"},
			},
		},
	}
}

func TestFilterMatchingNote(t *testing.T) {
	got := Filter("milk", sections())
	sec := got.Sections[0]
	if !sec.Visible {
		t.Error("section with a matching note: Visible = false, want true")
	}
	if sec.TitleMatched {
		t.Error("section title did not match: TitleMatched = true, want false")
	}
	if !sec.Notes[0].Matched || !sec.Notes[0].Show {
		t.Errorf("note[0] (matches) = %+v, want Matched && Show", sec.Notes[0])
	}
	if sec.Notes[1].Matched || sec.Notes[1].Show {
		t.Errorf("note[1] (does not match) = %+v, want neither Matched nor Show", sec.Notes[1])
	}
	if other := got.Sections[1]; other.Visible {
		t.Error("unrelated section: Visible = true, want false")
	}
}

func TestFilterMatchingTitleCascadesToEveryNote(t *testing.T) {
	got := Filter("groceries", sections())
	sec := got.Sections[0]
	if !sec.TitleMatched {
		t.Error("TitleMatched = false, want true")
	}
	if !sec.Visible {
		t.Error("Visible = false, want true")
	}
	for i, n := range sec.Notes {
		if !n.Show {
			t.Errorf("note[%d].Show = false, want true (title-matched section jump)", i)
		}
		if n.Matched {
			t.Errorf("note[%d].Matched = true, want false (only the title matched, not the body)", i)
		}
	}
}

func TestFilterNeitherMatches(t *testing.T) {
	got := Filter("zzz-nomatch", sections())
	for si, sec := range got.Sections {
		if sec.Visible {
			t.Errorf("section[%d].Visible = true, want false", si)
		}
		for ni, n := range sec.Notes {
			if n.Show {
				t.Errorf("section[%d].note[%d].Show = true, want false", si, ni)
			}
		}
	}
}

func TestFilterEmptyQueryShowsEverything(t *testing.T) {
	got := Filter("   ", sections())
	for si, sec := range got.Sections {
		if !sec.Visible {
			t.Errorf("section[%d].Visible = false, want true (empty query)", si)
		}
		if sec.TitleMatched {
			t.Errorf("section[%d].TitleMatched = true, want false (empty query)", si)
		}
		for ni, n := range sec.Notes {
			if !n.Show {
				t.Errorf("section[%d].note[%d].Show = false, want true (empty query)", si, ni)
			}
			if n.Matched {
				t.Errorf("section[%d].note[%d].Matched = true, want false (empty query)", si, ni)
			}
		}
	}
}
