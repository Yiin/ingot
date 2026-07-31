package fsstore

import (
	"github.com/Yiin/ingot/internal/store"
)

// removedNote is one note captured by a destructive operation before it
// left its section — the shared source data for both that operation's
// trash file and the single undo slot Undo can restore from. See
// snapshotRemovedLocked in helpers.go.
type removedNote struct {
	project store.ProjectID
	section store.SectionID
	title   string
	index   int
	note    store.Note
}

// undoGroup is every removedNote that shared one Section, in ascending
// original index order.
type undoGroup struct {
	section store.SectionID
	entries []removedNote
}

// undoState is Ingot's single level of undo: the most recent destructive
// operation, or nil if there is nothing to reverse. DeleteNotes and
// ClearDone are the only operations that arm it — MoveNotes, MergeNotes,
// and DeleteSection's note relocation don't, since none of them actually
// removes a note from the project (see notes.go and sections.go). Every
// other mutation, and every external reload/conflict/remove, clears it:
// see markDirty (persist.go) and the explicit clearUndoLocked calls in
// reload.go and projects.go.
type undoState struct {
	label  string
	groups []undoGroup
}

// groupRemovedBySection buckets removed — already in ascending document
// order per snapshotRemovedLocked — into one undoGroup per distinct
// Section, preserving that per-section ascending-index order.
func groupRemovedBySection(removed []removedNote) []undoGroup {
	var groups []undoGroup
	var lastSection store.SectionID
	have := false
	for _, rn := range removed {
		if !have || rn.section != lastSection {
			groups = append(groups, undoGroup{section: rn.section})
			lastSection = rn.section
			have = true
		}
		g := &groups[len(groups)-1]
		g.entries = append(g.entries, rn)
	}
	return groups
}

// clearUndoLocked drops any pending undo slot. Must be called with s.mu
// held.
func (s *fileStore) clearUndoLocked() {
	s.undo = nil
}

// setUndoLocked arms the single undo slot with label and every note
// removed just before this call, or clears it if removed is empty. Must
// be called with s.mu held, after the removal it describes has already
// happened — any earlier clearUndoLocked call along that path (e.g. from
// markDirty) is superseded by this one.
func (s *fileStore) setUndoLocked(label string, removed []removedNote) {
	if len(removed) == 0 {
		s.undo = nil
		return
	}
	s.undo = &undoState{label: label, groups: groupRemovedBySection(removed)}
}

func (s *fileStore) CanUndo() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.undo == nil {
		return "", false
	}
	return s.undo.label, true
}

func (s *fileStore) Undo() error {
	s.mu.Lock()
	events := s.undoLocked()
	s.mu.Unlock()

	s.emit(events...)
	return nil
}

// undoLocked restores every note in s.undo's groups to its original
// section and index, then consumes the slot regardless of outcome — a
// single level of undo, per invariant 14, never a stack. A group whose
// Section no longer exists (the section itself was since deleted, or its
// project removed) is skipped rather than failing the whole Undo: the
// rest of the slot may still be restorable. Must be called with s.mu
// held.
func (s *fileStore) undoLocked() []store.Event {
	if s.undo == nil {
		return nil
	}
	undo := s.undo
	s.undo = nil

	var events []store.Event
	touched := map[store.ProjectID]bool{}
	for _, g := range undo.groups {
		pi, si, ok := s.locateSection(g.section)
		if !ok {
			continue
		}
		pid := s.order[pi]
		pe := s.projects[pid]
		if pe.readOnly {
			events = append(events, store.ProjectReadOnly{})
			continue
		}

		notes := pe.proj.Sections[si].Notes
		for _, rn := range g.entries {
			idx := rn.index
			if idx > len(notes) {
				idx = len(notes)
			}
			widened := append(notes, store.Note{})
			copy(widened[idx+1:], widened[idx:])
			widened[idx] = rn.note
			notes = widened
			events = append(events, store.NotesSpliced{Project: pi, Section: si, Index: idx, Removed: 0, Added: 1})
		}
		pe.proj.Sections[si].Notes = notes
		touched[pid] = true
	}

	for pid := range touched {
		s.markDirty(pid)
		events = s.flushAndCollect(pid, events)
	}
	return events
}
