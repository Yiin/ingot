package fsstore

import (
	"strings"
	"time"

	"github.com/Yiin/ingot/internal/store"
)

func (s *fileStore) AddNote(section store.SectionID, body string) (store.NoteID, error) {
	normalized := normalizeBody(body)
	if strings.TrimSpace(normalized) == "" {
		return "", store.ErrEmptyBody
	}

	s.mu.Lock()
	id, events, err := s.addNoteLocked(section, normalized, store.SourceTyped, store.Origin{})
	s.mu.Unlock()

	s.emit(events...)
	return id, err
}

// AppendToDefault resolves the default capture location itself — the
// active project's last section — independent of whatever section a
// caller's UI happens to have open. Per invariant 4, there is no other
// notion of "active section" anywhere in the Store.
func (s *fileStore) AppendToDefault(body string, origin store.Origin) (store.NoteID, error) {
	normalized := normalizeBody(body)
	if strings.TrimSpace(normalized) == "" {
		return "", store.ErrEmptyBody
	}

	s.mu.Lock()
	pe, ok := s.projects[s.active]
	if !ok {
		s.mu.Unlock()
		return "", store.ErrNotFound
	}
	section := pe.proj.Sections[len(pe.proj.Sections)-1].ID
	id, events, err := s.addNoteLocked(section, normalized, store.SourceCaptured, origin)
	s.mu.Unlock()

	s.emit(events...)
	return id, err
}

// addNoteLocked appends one note to section; body is already normalized
// and already known non-empty. AddNote and AppendToDefault use the
// ordinary debounced save, not the immediate-flush path — losing up to
// Debounce worth of a just-added note on a crash is the same accepted
// risk as losing in-progress typing. Must be called with s.mu held.
func (s *fileStore) addNoteLocked(section store.SectionID, body string, src store.Source, origin store.Origin) (store.NoteID, []store.Event, error) {
	pi, si, ok := s.locateSection(section)
	if !ok {
		return "", nil, store.ErrNotFound
	}
	pid := s.order[pi]
	pe := s.projects[pid]
	if pe.readOnly {
		return "", []store.Event{store.ProjectReadOnly{}}, store.ErrReadOnly
	}

	note := store.Note{
		ID:      store.NoteID(s.newID()),
		Body:    body,
		Created: s.now(),
		Source:  src,
		App:     origin.App,
		URI:     origin.URI,
	}
	notes := pe.proj.Sections[si].Notes
	index := len(notes)
	pe.proj.Sections[si].Notes = append(notes, note)

	s.markDirty(pid)
	events := []store.Event{store.NotesSpliced{Project: pi, Section: si, Index: index, Removed: 0, Added: 1}}
	return note.ID, events, nil
}

func (s *fileStore) SetNoteBody(id store.NoteID, body string) error {
	normalized := normalizeBody(body)
	if strings.TrimSpace(normalized) == "" {
		return store.ErrEmptyBody
	}

	s.mu.Lock()
	events, err := s.setNoteBodyLocked(id, normalized)
	s.mu.Unlock()

	s.emit(events...)
	return err
}

func (s *fileStore) setNoteBodyLocked(id store.NoteID, body string) ([]store.Event, error) {
	pi, si, ni, ok := s.locateNote(id)
	if !ok {
		return nil, store.ErrNotFound
	}
	pid := s.order[pi]
	pe := s.projects[pid]
	if pe.readOnly {
		return []store.Event{store.ProjectReadOnly{}}, store.ErrReadOnly
	}

	pe.proj.Sections[si].Notes[ni].Body = body
	s.markDirty(pid)
	return []store.Event{store.NoteUpdated{Project: pi, Section: si, Index: ni, ID: id}}, nil
}

func (s *fileStore) SetNoteDone(id store.NoteID, done bool) error {
	s.mu.Lock()
	events, err := s.setNoteDoneLocked(id, done)
	s.mu.Unlock()

	s.emit(events...)
	return err
}

func (s *fileStore) setNoteDoneLocked(id store.NoteID, done bool) ([]store.Event, error) {
	pi, si, ni, ok := s.locateNote(id)
	if !ok {
		return nil, store.ErrNotFound
	}
	pid := s.order[pi]
	pe := s.projects[pid]
	if pe.readOnly {
		return []store.Event{store.ProjectReadOnly{}}, store.ErrReadOnly
	}

	note := &pe.proj.Sections[si].Notes[ni]
	note.Done = done
	if done {
		note.DoneAt = s.now()
	} else {
		note.DoneAt = time.Time{}
	}
	s.markDirty(pid)
	return []store.Event{store.NoteUpdated{Project: pi, Section: si, Index: ni, ID: id}}, nil
}

func (s *fileStore) DeleteNotes(ids []store.NoteID) error {
	ids = dedupeNoteIDs(ids)
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	events, err := s.deleteNotesLocked(ids)
	s.mu.Unlock()

	s.emit(events...)
	return err
}

func (s *fileStore) deleteNotesLocked(ids []store.NoteID) ([]store.Event, error) {
	locs, roEvents, err := s.resolveNoteLocs(ids)
	if err != nil {
		return roEvents, err
	}

	events := s.removeNoteLocsLocked(locs)
	events = s.flushTouchedLocked(locs, events)
	return events, nil
}

// ClearDone removes every done note from the active project across
// every section, never removing a section itself even if it becomes
// empty (invariant 7).
func (s *fileStore) ClearDone() error {
	s.mu.Lock()
	events, err := s.clearDoneLocked()
	s.mu.Unlock()

	s.emit(events...)
	return err
}

func (s *fileStore) clearDoneLocked() ([]store.Event, error) {
	pid := s.active
	pe, ok := s.projects[pid]
	if !ok {
		return nil, store.ErrNotFound
	}
	if pe.readOnly {
		return []store.Event{store.ProjectReadOnly{}}, store.ErrReadOnly
	}
	projIdx, _ := s.indexOfProject(pid)

	var locs []noteLoc
	for si, sec := range pe.proj.Sections {
		for ni, n := range sec.Notes {
			if n.Done {
				locs = append(locs, noteLoc{projIdx, si, ni})
			}
		}
	}
	if len(locs) == 0 {
		return nil, nil
	}

	events := s.removeNoteLocsLocked(locs)
	s.markDirty(pid)
	events = s.flushAndCollect(pid, events)
	return events, nil
}

func (s *fileStore) MoveNotes(ids []store.NoteID, toSection store.SectionID) error {
	ids = dedupeNoteIDs(ids)
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	events, err := s.moveNotesLocked(ids, toSection)
	s.mu.Unlock()

	s.emit(events...)
	return err
}

func (s *fileStore) moveNotesLocked(ids []store.NoteID, toSection store.SectionID) ([]store.Event, error) {
	dstProjIdx, dstSecIdx, ok := s.locateSection(toSection)
	if !ok {
		return nil, store.ErrNotFound
	}
	dstPID := s.order[dstProjIdx]
	dstPE := s.projects[dstPID]
	if dstPE.readOnly {
		return []store.Event{store.ProjectReadOnly{}}, store.ErrReadOnly
	}

	locs, roEvents, err := s.resolveNoteLocs(ids)
	if err != nil {
		return roEvents, err
	}

	// Snapshot the notes, in document order, before removeNoteLocsLocked
	// mutates the sections out from under loc.noteIdx.
	sorted := sortLocsByPosition(locs)
	moved := make([]store.Note, len(sorted))
	for i, loc := range sorted {
		moved[i] = s.projects[s.order[loc.projIdx]].proj.Sections[loc.secIdx].Notes[loc.noteIdx]
	}

	events := s.removeNoteLocsLocked(locs)

	dstNotes := dstPE.proj.Sections[dstSecIdx].Notes
	insertIndex := len(dstNotes)
	dstPE.proj.Sections[dstSecIdx].Notes = append(dstNotes, moved...)
	events = append(events, store.NotesSpliced{
		Project: dstProjIdx,
		Section: dstSecIdx,
		Index:   insertIndex,
		Removed: 0,
		Added:   len(moved),
	})

	locs = append(locs, noteLoc{projIdx: dstProjIdx})
	events = s.flushTouchedLocked(locs, events)
	return events, nil
}

func (s *fileStore) MergeNotes(ids []store.NoteID) (store.NoteID, error) {
	ids = dedupeNoteIDs(ids)
	if len(ids) < 2 {
		return "", store.ErrTooFewNotes
	}
	s.mu.Lock()
	id, events, err := s.mergeNotesLocked(ids)
	s.mu.Unlock()

	s.emit(events...)
	return id, err
}

func (s *fileStore) mergeNotesLocked(ids []store.NoteID) (store.NoteID, []store.Event, error) {
	locs, roEvents, err := s.resolveNoteLocs(ids)
	if err != nil {
		return "", roEvents, err
	}

	sorted := sortLocsByPosition(locs)
	bodies := make([]string, len(sorted))
	done := true
	var earliest time.Time
	for i, loc := range sorted {
		n := s.projects[s.order[loc.projIdx]].proj.Sections[loc.secIdx].Notes[loc.noteIdx]
		bodies[i] = n.Body
		if !n.Done {
			done = false
		}
		if earliest.IsZero() || n.Created.Before(earliest) {
			earliest = n.Created
		}
	}

	merged := store.Note{
		ID:      store.NoteID(s.newID()),
		Body:    normalizeBody(strings.Join(bodies, "\n\n")),
		Done:    done,
		Created: earliest,
		Source:  store.SourceMerged,
	}
	if done {
		merged.DoneAt = s.now()
	}

	// The document-order-first input's own slot survives the removal
	// below untouched, since every other removed index in its section
	// is >= it — so its original position is exactly where the merged
	// note belongs once its own entry (and everything after it that's
	// also being merged) is gone.
	first := sorted[0]
	events := s.removeNoteLocsLocked(locs)

	pe := s.projects[s.order[first.projIdx]]
	notes := pe.proj.Sections[first.secIdx].Notes
	insertIdx := first.noteIdx
	if insertIdx > len(notes) {
		insertIdx = len(notes)
	}
	widened := append(notes, store.Note{})
	copy(widened[insertIdx+1:], widened[insertIdx:])
	widened[insertIdx] = merged
	pe.proj.Sections[first.secIdx].Notes = widened

	events = append(events, store.NotesSpliced{
		Project: first.projIdx,
		Section: first.secIdx,
		Index:   insertIdx,
		Removed: 0,
		Added:   1,
	})

	events = s.flushTouchedLocked(locs, events)
	return merged.ID, events, nil
}

// resolveNoteLocs locates every id, failing the whole call with
// ErrNotFound if any doesn't resolve, or ErrReadOnly (plus a
// ProjectReadOnly event) if any resolves into a read-only project.
// Must be called with s.mu held.
func (s *fileStore) resolveNoteLocs(ids []store.NoteID) ([]noteLoc, []store.Event, error) {
	locs := make([]noteLoc, len(ids))
	for i, id := range ids {
		pi, si, ni, ok := s.locateNote(id)
		if !ok {
			return nil, nil, store.ErrNotFound
		}
		locs[i] = noteLoc{pi, si, ni}
	}
	for _, loc := range locs {
		if s.projects[s.order[loc.projIdx]].readOnly {
			return nil, []store.Event{store.ProjectReadOnly{}}, store.ErrReadOnly
		}
	}
	return locs, nil, nil
}

// flushTouchedLocked marks dirty and immediately flushes — bypassing
// the debounce — every project referenced by locs, appending SaveFailed
// to events for any that fails. This is the "structural mutation"
// immediate-flush path: DeleteNotes, MoveNotes, and MergeNotes all use
// it. Must be called with s.mu held.
func (s *fileStore) flushTouchedLocked(locs []noteLoc, events []store.Event) []store.Event {
	touched := map[store.ProjectID]bool{}
	for _, loc := range locs {
		touched[s.order[loc.projIdx]] = true
	}
	for pid := range touched {
		s.markDirty(pid)
		events = s.flushAndCollect(pid, events)
	}
	return events
}
