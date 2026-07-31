package fsstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/paths"
)

// fileStore is the store.Store implementation. It is not goroutine-safe
// from the caller's perspective — see package store's doc comment — but
// guards its own state with a mutex regardless, since a debounced save
// fired from a background timer (see persist.go) reaches back into that
// state from a different goroutine than whichever one is calling Store
// methods.
type fileStore struct {
	mu sync.Mutex

	fs       fsx.FS
	paths    paths.Layout
	now      func() time.Time
	newID    func() string
	post     func(func())
	debounce time.Duration
	maxDelay time.Duration

	projects map[store.ProjectID]*projectEntry
	// order is the discovery/creation order of every known project — the
	// same ordering Projects() returns and the same coordinate space
	// NotesSpliced/NoteUpdated's Project field indexes into.
	order  []store.ProjectID
	active store.ProjectID

	subs      []subEntry
	nextSubID int

	// searchCache holds each note's last-computed searchtext.Normalized
	// body, keyed by NoteID — see search.go's normalizedForLocked.
	searchCache map[store.NoteID]normCacheEntry

	closed bool
}

// New loads every projects/*.md under opts.Paths and returns a ready
// Store. See the package doc comment for the threading rule every
// method depends on.
func New(opts Options) (store.Store, error) {
	opts = opts.withDefaults()
	if opts.FS == nil {
		return nil, fmt.Errorf("fsstore: Options.FS is required")
	}

	s := &fileStore{
		fs:       opts.FS,
		paths:    opts.Paths,
		now:      opts.Now,
		newID:    opts.NewID,
		post:     opts.Post,
		debounce: opts.Debounce,
		maxDelay: opts.MaxDelay,
		projects: make(map[store.ProjectID]*projectEntry),
	}

	if s.paths.Projects != "" {
		if err := s.fs.MkdirAll(s.paths.Projects, 0o755); err != nil {
			return nil, fmt.Errorf("fsstore: create projects dir: %w", err)
		}
	}
	if s.paths.State != "" {
		if err := s.fs.MkdirAll(s.paths.State, 0o755); err != nil {
			return nil, fmt.Errorf("fsstore: create state dir: %w", err)
		}
	}

	if err := s.load(); err != nil {
		return nil, err
	}
	s.resolveActive()

	return s, nil
}

func (s *fileStore) Projects() []store.ProjectRef {
	s.mu.Lock()
	defer s.mu.Unlock()

	refs := make([]store.ProjectRef, 0, len(s.order))
	for _, id := range s.order {
		pe := s.projects[id]
		notes, done := 0, 0
		for _, sec := range pe.proj.Sections {
			notes += len(sec.Notes)
			for _, n := range sec.Notes {
				if n.Done {
					done++
				}
			}
		}
		refs = append(refs, store.ProjectRef{
			ID:    id,
			Title: pe.proj.Title,
			Path:  pe.path,
			Notes: notes,
			Done:  done,
		})
	}
	return refs
}

func (s *fileStore) Project(id store.ProjectID) (store.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pe, ok := s.projects[id]
	if !ok {
		return store.Project{}, store.ErrNotFound
	}
	return cloneProject(pe.proj), nil
}

func (s *fileStore) Active() store.ProjectID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func (s *fileStore) Note(id store.NoteID) (store.Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pi, si, ni, ok := s.locateNote(id)
	if !ok {
		return store.Note{}, store.ErrNotFound
	}
	return s.projects[s.order[pi]].proj.Sections[si].Notes[ni], nil
}

// CanUndo always reports false: single-level undo is a separate package
// addition (internal/store/fsstore's file-watcher/trash/undo child).
func (s *fileStore) CanUndo() bool { return false }

// Undo is a no-op until undo lands. See CanUndo.
func (s *fileStore) Undo() error { return nil }

func (s *fileStore) Subscribe(fn func(store.Event)) func() {
	s.mu.Lock()
	id := s.nextSubID
	s.nextSubID++
	s.subs = append(s.subs, subEntry{id: id, fn: fn})
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, sub := range s.subs {
			if sub.id == id {
				s.subs = append(s.subs[:i], s.subs[i+1:]...)
				return
			}
		}
	}
}

// emit fires every event to every currently-subscribed callback, in
// event order, on the calling goroutine — never while s.mu is held, so
// a subscriber that calls back into the Store doesn't deadlock on its
// own reentrant Lock.
func (s *fileStore) emit(events ...store.Event) {
	if len(events) == 0 {
		return
	}
	s.mu.Lock()
	subs := make([]subEntry, len(s.subs))
	copy(subs, s.subs)
	s.mu.Unlock()

	for _, ev := range events {
		for _, sub := range subs {
			sub.fn(ev)
		}
	}
}

func (s *fileStore) Flush(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	var firstErr error
	for _, id := range s.order {
		if err := s.flushLocked(id); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *fileStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	// Set before the flush loop, not after: flushLocked's own
	// write-failure retry checks s.closed, and a Close-triggered flush
	// attempt must not arm a timer nobody will be listening for once
	// this call returns.
	s.closed = true
	var firstErr error
	for _, id := range s.order {
		if err := s.flushLocked(id); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
