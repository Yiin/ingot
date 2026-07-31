package fsstore

import "github.com/Yiin/ingot/internal/store"

func (s *fileStore) AddSection(project store.ProjectID, title string) (store.SectionID, error) {
	s.mu.Lock()
	id, events, err := s.addSectionLocked(project, title)
	s.mu.Unlock()

	s.emit(events...)
	return id, err
}

func (s *fileStore) addSectionLocked(project store.ProjectID, title string) (store.SectionID, []store.Event, error) {
	pe, ok := s.projects[project]
	if !ok {
		return "", nil, store.ErrNotFound
	}
	if pe.readOnly {
		return "", []store.Event{store.ProjectReadOnly{}}, store.ErrReadOnly
	}
	for _, sec := range pe.proj.Sections {
		if sec.Title == title {
			return "", nil, store.ErrDuplicateSection
		}
	}

	id := store.SectionID(s.newID())
	pe.proj.Sections = append(pe.proj.Sections, store.Section{ID: id, Title: title})

	s.markDirty(project)
	events := s.flushAndCollect(project, []store.Event{store.SectionsChanged{}})
	return id, events, nil
}

func (s *fileStore) RenameSection(id store.SectionID, title string) error {
	s.mu.Lock()
	events, err := s.renameSectionLocked(id, title)
	s.mu.Unlock()

	s.emit(events...)
	return err
}

func (s *fileStore) renameSectionLocked(id store.SectionID, title string) ([]store.Event, error) {
	pi, si, ok := s.locateSection(id)
	if !ok {
		return nil, store.ErrNotFound
	}
	pid := s.order[pi]
	pe := s.projects[pid]
	if pe.readOnly {
		return []store.Event{store.ProjectReadOnly{}}, store.ErrReadOnly
	}
	for i, sec := range pe.proj.Sections {
		if i != si && sec.Title == title {
			return nil, store.ErrDuplicateSection
		}
	}

	pe.proj.Sections[si].Title = title
	s.markDirty(pid)
	return s.flushAndCollect(pid, []store.Event{store.SectionsChanged{}}), nil
}

// DeleteSection removes a section. Its notes are never deleted
// (invariant 8): they move to the preceding section, appended to its
// end so document order is preserved, or — if the deleted section was
// first — to the following section, prepended to its start for the same
// reason.
func (s *fileStore) DeleteSection(id store.SectionID) error {
	s.mu.Lock()
	events, err := s.deleteSectionLocked(id)
	s.mu.Unlock()

	s.emit(events...)
	return err
}

func (s *fileStore) deleteSectionLocked(id store.SectionID) ([]store.Event, error) {
	pi, si, ok := s.locateSection(id)
	if !ok {
		return nil, store.ErrNotFound
	}
	pid := s.order[pi]
	pe := s.projects[pid]
	if pe.readOnly {
		return []store.Event{store.ProjectReadOnly{}}, store.ErrReadOnly
	}
	sections := pe.proj.Sections
	if len(sections) == 1 {
		return nil, store.ErrLastSection
	}

	var events []store.Event
	removed := sections[si].Notes
	if len(removed) > 0 {
		// The section's grouping is genuinely destroyed even though its
		// notes survive by relocating — record what it held, same as
		// every other destructive op, before the relocation below makes
		// that grouping unrecoverable.
		if _, err := s.writeTrashLocked(pid, "delete-section", []trashSection{{title: sections[si].Title, notes: removed}}); err != nil {
			events = append(events, store.SaveFailed{})
		}

		if si > 0 {
			prev := &sections[si-1]
			prev.Notes = append(prev.Notes, removed...)
		} else {
			next := &sections[si+1]
			next.Notes = append(append([]store.Note(nil), removed...), next.Notes...)
		}
	}

	pe.proj.Sections = append(sections[:si], sections[si+1:]...)

	s.markDirty(pid)
	events = append(events, s.flushAndCollect(pid, []store.Event{store.SectionsChanged{}})...)
	return events, nil
}

func (s *fileStore) MoveSection(id store.SectionID, index int) error {
	s.mu.Lock()
	events, err := s.moveSectionLocked(id, index)
	s.mu.Unlock()

	s.emit(events...)
	return err
}

func (s *fileStore) moveSectionLocked(id store.SectionID, index int) ([]store.Event, error) {
	pi, si, ok := s.locateSection(id)
	if !ok {
		return nil, store.ErrNotFound
	}
	pid := s.order[pi]
	pe := s.projects[pid]
	if pe.readOnly {
		return []store.Event{store.ProjectReadOnly{}}, store.ErrReadOnly
	}

	sections := pe.proj.Sections
	sec := sections[si]
	sections = append(sections[:si], sections[si+1:]...)

	if index < 0 {
		index = 0
	}
	if index > len(sections) {
		index = len(sections)
	}
	sections = append(sections, store.Section{})
	copy(sections[index+1:], sections[index:])
	sections[index] = sec
	pe.proj.Sections = sections

	s.markDirty(pid)
	return s.flushAndCollect(pid, []store.Event{store.SectionsChanged{}}), nil
}
