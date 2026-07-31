package searchtext

import "github.com/Yiin/ingot/internal/store"

// NoteFilter describes one note's visibility for a search.
type NoteFilter struct {
	// Matched is true when the note's own body matched the query.
	Matched bool
	// Show is true when the note should be visible in the panel:
	// Matched, or its section's title matched (see SectionFilter.
	// TitleMatched).
	Show bool
	// Ranges gives raw-body byte offsets to highlight, non-empty only
	// when Matched.
	Ranges [][2]int
}

// SectionFilter describes one section's visibility for a search.
type SectionFilter struct {
	// TitleMatched is true when the section's own title matched the
	// query. This turns the search field into a section jump: every
	// note in the section is Show, regardless of whether its own body
	// matched.
	TitleMatched bool
	// Visible is true when the section should be shown at all: any of
	// its notes Show, or TitleMatched.
	Visible bool
	// Notes is parallel to the Section's Notes.
	Notes []NoteFilter
}

// FilterResult is Filter's output, parallel to a Project's Sections.
type FilterResult struct {
	Sections []SectionFilter
}

// Filter computes, for query against sections, which section headers
// and which notes the panel should show — a header is visible when any
// of its notes match or its own title matches, and a matching title
// cascades visibility to every note in that section.
//
// An empty query (nothing left after Tokens splits and folds it) shows
// everything: every section Visible, every note Show, nothing reported
// Matched — the "no active filter" state.
//
// Filter calls Normalize directly on every title and body, so a caller
// that already holds fsstore's per-note cache (see fsstore/search.go)
// gets no benefit from it here — fine for this child's own tests, but
// worth revisiting before a live per-keystroke UI (e.g. Filter accepting
// pre-normalized inputs) consumes this on a large project.
func Filter(query string, sections []store.Section) FilterResult {
	tokens := Tokens(query)
	result := FilterResult{Sections: make([]SectionFilter, len(sections))}

	for si, sec := range sections {
		notes := make([]NoteFilter, len(sec.Notes))

		if len(tokens) == 0 {
			for ni := range sec.Notes {
				notes[ni] = NoteFilter{Show: true}
			}
			result.Sections[si] = SectionFilter{Visible: true, Notes: notes}
			continue
		}

		titleMatched, _ := Normalize(sec.Title).Match(tokens)
		anyShown := titleMatched
		for ni, note := range sec.Notes {
			matched, ranges := Normalize(note.Body).Match(tokens)
			if matched {
				anyShown = true
			}
			notes[ni] = NoteFilter{
				Matched: matched,
				Show:    matched || titleMatched,
				Ranges:  ranges,
			}
		}
		result.Sections[si] = SectionFilter{
			TitleMatched: titleMatched,
			Visible:      anyShown,
			Notes:        notes,
		}
	}
	return result
}
