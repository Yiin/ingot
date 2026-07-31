package fsstore

import (
	"sort"
	"strings"

	"github.com/Yiin/ingot/internal/store"
)

// cloneProject deep-copies p's Sections/Notes and Extra so a caller of
// Project can freely mutate what it gets back without touching the
// Store's own state.
func cloneProject(p store.Project) store.Project {
	out := p
	out.Sections = make([]store.Section, len(p.Sections))
	for i, sec := range p.Sections {
		out.Sections[i] = sec
		out.Sections[i].Notes = append([]store.Note(nil), sec.Notes...)
	}
	if p.Extra != nil {
		out.Extra = make(map[string]string, len(p.Extra))
		for k, v := range p.Extra {
			out.Extra[k] = v
		}
	}
	return out
}

// normalizeBody canonicalizes a note body per invariant 12: CRLF and a
// lone CR collapse to \n, and leading/trailing blank lines are trimmed.
// Internal blank lines are left alone.
func normalizeBody(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")

	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.Join(lines[start:end], "\n")
}

// locateSection finds a section by id across every project, returning
// its position as an index into s.order and into that project's
// Sections slice — the same coordinates NotesSpliced and SectionsChanged
// subscribers use to address one. Must be called with s.mu held.
func (s *fileStore) locateSection(id store.SectionID) (projIdx, secIdx int, ok bool) {
	for pi, pid := range s.order {
		sections := s.projects[pid].proj.Sections
		for si := range sections {
			if sections[si].ID == id {
				return pi, si, true
			}
		}
	}
	return 0, 0, false
}

// locateNote finds a note by id across every project and section. Must
// be called with s.mu held.
func (s *fileStore) locateNote(id store.NoteID) (projIdx, secIdx, noteIdx int, ok bool) {
	for pi, pid := range s.order {
		sections := s.projects[pid].proj.Sections
		for si := range sections {
			notes := sections[si].Notes
			for ni := range notes {
				if notes[ni].ID == id {
					return pi, si, ni, true
				}
			}
		}
	}
	return 0, 0, 0, false
}

// indexOfProject returns id's position in s.order. Must be called with
// s.mu held.
func (s *fileStore) indexOfProject(id store.ProjectID) (int, bool) {
	for i, pid := range s.order {
		if pid == id {
			return i, true
		}
	}
	return 0, false
}

// dedupeNoteIDs drops repeats, keeping first occurrence order. Every
// caller-supplied []NoteID (DeleteNotes, MoveNotes, MergeNotes) goes
// through this before resolving locations: a repeated id would
// otherwise resolve to the same (section, index) twice, and removing
// that position twice deletes whatever note happens to sit there after
// the first removal instead — silent data loss, not a no-op.
func dedupeNoteIDs(ids []store.NoteID) []store.NoteID {
	seen := make(map[store.NoteID]bool, len(ids))
	out := make([]store.NoteID, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// noteLoc locates a single note within a multi-note operation
// (DeleteNotes, MoveNotes, MergeNotes, ClearDone) by the same
// (project-order-index, section-index, note-index) coordinates
// locateNote and locateSection use.
type noteLoc struct {
	projIdx, secIdx, noteIdx int
}

// run is a contiguous, zero-based span [start, start+length) of a
// Section's Notes slated for removal.
type run struct{ start, length int }

// runsFromIndices groups arbitrary note indices into ascending,
// non-overlapping contiguous runs, so a caller can turn a scattered
// selection into the minimum number of NotesSpliced removal events.
func runsFromIndices(idxs []int) []run {
	if len(idxs) == 0 {
		return nil
	}
	sorted := append([]int(nil), idxs...)
	sort.Ints(sorted)

	var runs []run
	i := 0
	for i < len(sorted) {
		j := i
		for j+1 < len(sorted) && sorted[j+1] == sorted[j]+1 {
			j++
		}
		runs = append(runs, run{start: sorted[i], length: j - i + 1})
		i = j + 1
	}
	return runs
}

// removeRuns deletes every run from notes, processing the
// highest-starting run first so removing one run never invalidates
// another run's already-computed start index.
func removeRuns(notes []store.Note, runs []run) []store.Note {
	for i := len(runs) - 1; i >= 0; i-- {
		r := runs[i]
		notes = append(notes[:r.start], notes[r.start+r.length:]...)
	}
	return notes
}

// removeNoteLocsLocked removes every located note from its section,
// grouping by (project, section) and by contiguous run so that each
// group produces the minimum number of NotesSpliced removal events, in
// ascending (project, section, position) order. Must be called with
// s.mu held.
func (s *fileStore) removeNoteLocsLocked(locs []noteLoc) []store.Event {
	// Evict before any mutation below, while loc.noteIdx still points at
	// the note it named.
	for _, loc := range locs {
		id := s.projects[s.order[loc.projIdx]].proj.Sections[loc.secIdx].Notes[loc.noteIdx].ID
		s.evictSearchCacheLocked(id)
	}

	type key struct{ projIdx, secIdx int }
	groups := map[key][]int{}
	for _, loc := range locs {
		k := key{loc.projIdx, loc.secIdx}
		groups[k] = append(groups[k], loc.noteIdx)
	}

	keys := make([]key, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].projIdx != keys[j].projIdx {
			return keys[i].projIdx < keys[j].projIdx
		}
		return keys[i].secIdx < keys[j].secIdx
	})

	var events []store.Event
	for _, k := range keys {
		pe := s.projects[s.order[k.projIdx]]
		runs := runsFromIndices(groups[k])
		pe.proj.Sections[k.secIdx].Notes = removeRuns(pe.proj.Sections[k.secIdx].Notes, runs)
		// Emit highest-start-first, mirroring the order removeRuns
		// actually applied them in. NotesSpliced maps 1:1 onto
		// gio.ListModel.ItemsChanged, which a subscriber applies
		// sequentially against its own copy of the list — emitting a
		// lower run's original index before a higher run's has already
		// been removed would have the subscriber splice at a position
		// that's shifted out from under it.
		for i := len(runs) - 1; i >= 0; i-- {
			r := runs[i]
			events = append(events, store.NotesSpliced{
				Project: k.projIdx,
				Section: k.secIdx,
				Index:   r.start,
				Removed: r.length,
				Added:   0,
			})
		}
	}
	return events
}

// sortLocsByPosition returns a copy of locs in ascending document order
// — (project, section, position) — independent of the order the caller
// originally supplied them in. MoveNotes and MergeNotes both use this:
// document order, not click/argument order, is what stays visible in
// the result.
func sortLocsByPosition(locs []noteLoc) []noteLoc {
	sorted := append([]noteLoc(nil), locs...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.projIdx != b.projIdx {
			return a.projIdx < b.projIdx
		}
		if a.secIdx != b.secIdx {
			return a.secIdx < b.secIdx
		}
		return a.noteIdx < b.noteIdx
	})
	return sorted
}
