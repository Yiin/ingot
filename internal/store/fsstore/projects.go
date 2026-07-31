package fsstore

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/mdfile"
	"github.com/Yiin/ingot/internal/store/paths"
)

func (s *fileStore) SetActive(id store.ProjectID) error {
	s.mu.Lock()
	events, err := s.setActiveLocked(id)
	s.mu.Unlock()

	s.emit(events...)
	return err
}

func (s *fileStore) setActiveLocked(id store.ProjectID) ([]store.Event, error) {
	if _, ok := s.projects[id]; !ok {
		return nil, store.ErrNotFound
	}
	if id == s.active {
		return nil, nil
	}
	s.active = id

	events := []store.Event{store.ActiveProjectChanged{}}
	if err := s.writeStateLocked(); err != nil {
		events = append(events, store.SaveFailed{})
	}
	return events, nil
}

func (s *fileStore) CreateProject(title string) (store.ProjectID, error) {
	s.mu.Lock()
	id, events, err := s.createProjectLocked(title)
	s.mu.Unlock()

	s.emit(events...)
	return id, err
}

func (s *fileStore) createProjectLocked(title string) (store.ProjectID, []store.Event, error) {
	for _, pid := range s.order {
		if s.projects[pid].proj.Title == title {
			return "", nil, store.ErrNameTaken
		}
	}

	existingSlugs := make([]string, 0, len(s.order))
	for _, pid := range s.order {
		existingSlugs = append(existingSlugs, s.projects[pid].slug)
	}
	slug := paths.UniqueSlug(existingSlugs, paths.Slug(title))
	path, err := paths.ProjectFile(s.paths, slug)
	if err != nil {
		return "", nil, fmt.Errorf("fsstore: create project: %w", err)
	}

	id := store.ProjectID(s.newID())
	proj := store.Project{
		ID:      id,
		Title:   title,
		Created: s.now(),
		Schema:  mdfile.CurrentSchema,
		Sections: []store.Section{
			{ID: store.SectionID(s.newID())},
		},
	}

	s.projects[id] = &projectEntry{proj: proj, path: path, slug: slug}
	s.order = append(s.order, id)

	events := []store.Event{store.ProjectListChanged{}}
	// A store with projects but no active one is a dead end for
	// AppendToDefault and ClearDone, both scoped to Active(); give the
	// very first project the same "always something active when
	// something exists" default resolveActive applies at load.
	if s.active == "" {
		s.active = id
		events = append(events, store.ActiveProjectChanged{})
		if err := s.writeStateLocked(); err != nil {
			events = append(events, store.SaveFailed{})
		}
	}

	s.markDirty(id)
	events = s.flushAndCollect(id, events)
	return id, events, nil
}

func (s *fileStore) RenameProject(id store.ProjectID, title string) error {
	s.mu.Lock()
	events, err := s.renameProjectLocked(id, title)
	s.mu.Unlock()

	s.emit(events...)
	return err
}

func (s *fileStore) renameProjectLocked(id store.ProjectID, title string) ([]store.Event, error) {
	pe, ok := s.projects[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	if pe.readOnly {
		return []store.Event{store.ProjectReadOnly{}}, store.ErrReadOnly
	}
	for _, pid := range s.order {
		if pid != id && s.projects[pid].proj.Title == title {
			return nil, store.ErrNameTaken
		}
	}

	pe.proj.Title = title
	s.markDirty(id)
	return s.flushAndCollect(id, []store.Event{store.ProjectListChanged{}}), nil
}

// DeleteProject removes a project's in-memory state and its file. A
// read-only project may still be deleted — removal doesn't risk
// clobbering content the Store didn't understand, only reading and
// rewriting it does.
func (s *fileStore) DeleteProject(id store.ProjectID) error {
	s.mu.Lock()
	events, err := s.deleteProjectLocked(id)
	s.mu.Unlock()

	s.emit(events...)
	return err
}

func (s *fileStore) deleteProjectLocked(id store.ProjectID) ([]store.Event, error) {
	pe, ok := s.projects[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	if pe.idleTimer != nil {
		pe.idleTimer.Stop()
	}
	if pe.maxTimer != nil {
		pe.maxTimer.Stop()
	}

	for _, sec := range pe.proj.Sections {
		for _, n := range sec.Notes {
			s.evictSearchCacheLocked(n.ID)
		}
	}

	idx, _ := s.indexOfProject(id)
	s.order = append(s.order[:idx], s.order[idx+1:]...)
	delete(s.projects, id)

	events := []store.Event{store.ProjectListChanged{}}
	if s.active == id {
		if len(s.order) > 0 {
			s.active = s.order[0]
		} else {
			s.active = ""
		}
		events = append(events, store.ActiveProjectChanged{})
		if err := s.writeStateLocked(); err != nil {
			events = append(events, store.SaveFailed{})
		}
	}

	if err := s.fs.Remove(pe.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return events, fmt.Errorf("fsstore: delete project: %w", err)
	}
	return events, nil
}
