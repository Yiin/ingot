package fsstore

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/mdfile"
)

// stateFile is the shape of $XDG_STATE_HOME/ingot/state.json.
type stateFile struct {
	ActiveProject string `json:"activeProject"`
}

func (s *fileStore) statePath() string {
	if s.paths.State == "" {
		return ""
	}
	return filepath.Join(s.paths.State, "state.json")
}

// resolveActive sets s.active from state.json if it names a project
// that's actually loaded, otherwise falls back to the first project in
// discovery order, or "" if there are none.
func (s *fileStore) resolveActive() {
	if path := s.statePath(); path != "" {
		if raw, err := s.fs.ReadFile(path); err == nil {
			var st stateFile
			if json.Unmarshal(raw, &st) == nil {
				if _, ok := s.projects[store.ProjectID(st.ActiveProject)]; ok {
					s.active = store.ProjectID(st.ActiveProject)
					return
				}
			}
		}
	}
	if len(s.order) > 0 {
		s.active = s.order[0]
	}
}

// writeStateLocked persists s.active to state.json. Must be called with
// s.mu held.
func (s *fileStore) writeStateLocked() error {
	path := s.statePath()
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(stateFile{ActiveProject: string(s.active)}, "", "  ")
	if err != nil {
		return err
	}
	return fsx.AtomicWrite(s.fs, path, data)
}

// markDirty arms id's debounce timers: idleTimer resets on every call,
// firing Debounce after the most recent mutation; maxTimer is set only
// on the transition into "dirty" and is never reset, capping the delay
// at MaxDelay regardless of how often idleTimer keeps getting pushed
// back by a continuous stream of edits. Must be called with s.mu held.
func (s *fileStore) markDirty(id store.ProjectID) {
	pe := s.projects[id]
	if pe == nil || pe.readOnly {
		return
	}

	if !pe.dirty {
		pe.dirty = true
		pe.maxTimer = time.AfterFunc(s.maxDelay, func() { s.post(func() { s.doFlush(id) }) })
	}
	if pe.idleTimer != nil {
		pe.idleTimer.Stop()
	}
	pe.idleTimer = time.AfterFunc(s.debounce, func() { s.post(func() { s.doFlush(id) }) })
}

// doFlush is the target of both debounce timers, always reached through
// Post so it runs on the goroutine that constructed the Store. A timer
// can still be in flight — already fired, its Post callback merely not
// yet run — at the instant Close stops accepting new work; bail out
// rather than write to (or retry against) a Store the caller was told
// is done with.
func (s *fileStore) doFlush(id store.ProjectID) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	err := s.flushLocked(id)
	s.mu.Unlock()
	if err != nil {
		s.emit(store.SaveFailed{})
	}
}

// flushLocked cancels id's timers and, if it's dirty, writes it — unless
// it's read-only, or the freshly formatted bytes are identical to what
// was last written, in which case the write is skipped entirely. On a
// write failure the project is left dirty and, unless the Store is
// closing, a fresh idle timer is armed so the Store keeps retrying, per
// Store.Flush's doc — Close arms no such timer, since nothing will ever
// be there to catch its eventual SaveFailed. Must be called with s.mu
// held.
func (s *fileStore) flushLocked(id store.ProjectID) error {
	pe := s.projects[id]
	if pe == nil {
		return nil
	}

	if pe.idleTimer != nil {
		pe.idleTimer.Stop()
		pe.idleTimer = nil
	}
	if pe.maxTimer != nil {
		pe.maxTimer.Stop()
		pe.maxTimer = nil
	}
	if !pe.dirty {
		return nil
	}
	pe.dirty = false
	if pe.readOnly {
		return nil
	}

	data, err := mdfile.Format(pe.proj)
	if err != nil {
		return err
	}
	if bytes.Equal(data, pe.lastWritten) {
		return nil
	}
	if err := fsx.AtomicWrite(s.fs, pe.path, data); err != nil {
		pe.dirty = true
		if !s.closed {
			pe.idleTimer = time.AfterFunc(s.debounce, func() { s.post(func() { s.doFlush(id) }) })
		}
		return err
	}
	pe.lastWritten = data
	return nil
}

// flushAndCollect flushes id immediately — bypassing the debounce, for
// every mutation invariant 14's "Flush immediately" list names — and
// appends SaveFailed to events if that write failed. Must be called
// with s.mu held.
func (s *fileStore) flushAndCollect(id store.ProjectID, events []store.Event) []store.Event {
	if err := s.flushLocked(id); err != nil {
		events = append(events, store.SaveFailed{})
	}
	return events
}
