package fsstore

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/mdfile"
	"github.com/Yiin/ingot/internal/store/paths"
	"github.com/Yiin/ingot/internal/store/provenance"
)

// metaPath returns slug's provenance sidecar path, or "" if no Meta
// directory is configured — the same "feature absent" convention
// statePath uses for Paths.State.
func (s *fileStore) metaPath(slug string) string {
	if s.paths.Meta == "" {
		return ""
	}
	p, err := paths.MetaFile(s.paths, slug)
	if err != nil {
		return ""
	}
	return p
}

// writeMetaLocked writes (or, once empty, deletes) pe's provenance
// sidecar. Failures are swallowed rather than surfaced as SaveFailed:
// the sidecar is advisory only and must never be required for correct
// operation, so a write failure here may only cost provenance on the
// next reload, never block or retry the project's own save.
func (s *fileStore) writeMetaLocked(pe *projectEntry) {
	path := s.metaPath(pe.slug)
	if path == "" {
		return
	}
	_ = provenance.Save(s.fs, path, provenance.Extract(pe.proj.Sections))
}

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
// back by a continuous stream of edits. It's also the one choke point
// every mutating method funnels a state-committing change through, so
// it doubles as where the single-level undo slot gets invalidated: per
// invariant 14, "any subsequent mutation clears the slot." DeleteNotes
// and ClearDone re-arm it with setUndoLocked immediately afterward,
// which simply supersedes this clear. Must be called with s.mu held.
func (s *fileStore) markDirty(id store.ProjectID) {
	s.clearUndoLocked()

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
	events, err := s.flushLocked(id)
	s.mu.Unlock()
	if err != nil {
		events = append(events, store.SaveFailed{})
	}
	s.emit(events...)
}

// flushLocked cancels id's timers and, if it's dirty, writes it — unless
// it's read-only, in which case the write is skipped entirely. Before
// writing, it checks pe.path's live on-disk state against pe's last
// known fingerprint (see detectExternalChangeLocked): a mismatch means
// something wrote to the file outside this Store since the fingerprint
// was recorded, which the file watcher's own 200ms debounce may not
// have settled on yet. flushLocked is every debounced save's and every
// immediate-flush structural mutation's write path — the hottest one in
// the package — so without this check a save fired in that gap would
// silently clobber the external write via AtomicWrite before the
// watcher ever saw it happened. When no conflict is detected, the
// original short-circuit still applies: freshly formatted bytes
// identical to what was last written skip the write outright. On a
// write failure (or a failure to even check for a conflict) the project
// is left dirty and, unless the Store is closing, a fresh idle timer is
// armed so the Store keeps retrying, per Store.Flush's doc — Close arms
// no such timer, since nothing will ever be there to catch its eventual
// SaveFailed. Must be called with s.mu held.
func (s *fileStore) flushLocked(id store.ProjectID) ([]store.Event, error) {
	pe := s.projects[id]
	if pe == nil {
		return nil, nil
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
		return nil, nil
	}
	pe.dirty = false
	if pe.readOnly {
		return nil, nil
	}

	data, err := mdfile.Format(pe.proj)
	if err != nil {
		return nil, err
	}

	trashPath, conflict, err := s.detectExternalChangeLocked(pe)
	if err != nil {
		pe.dirty = true
		if !s.closed {
			pe.idleTimer = time.AfterFunc(s.debounce, func() { s.post(func() { s.doFlush(id) }) })
		}
		return nil, err
	}
	if !conflict && bytes.Equal(data, pe.lastWritten) {
		return nil, nil
	}

	if err := s.writeAndRecordLocked(id, pe, data); err != nil {
		return nil, err
	}
	if conflict {
		return []store.Event{store.ConflictResolved{ID: id, SavedTo: trashPath}}, nil
	}
	return nil, nil
}

// flushForceLocked writes id's current in-memory content to disk
// unconditionally, bypassing both of flushLocked's short-circuits
// ("not dirty" and "unchanged since pe.lastWritten"). Those compare
// against pe.lastWritten, which describes what this Store itself last
// wrote — not necessarily what's on disk right now.
// resolveConflictLocked (reload.go) needs this: it must physically
// overwrite whatever an external process just put at pe.path even when
// that content happens to format identically to our own last-known
// write, or the "panel wins" conflict policy silently loses on a
// coincidental byte match. Must be called with s.mu held.
func (s *fileStore) flushForceLocked(id store.ProjectID) error {
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
	pe.dirty = false
	if pe.readOnly {
		return nil
	}

	data, err := mdfile.Format(pe.proj)
	if err != nil {
		return err
	}
	return s.writeAndRecordLocked(id, pe, data)
}

// writeAndRecordLocked is flushLocked/flushForceLocked's shared tail:
// atomically write data to pe.path, and on success adopt it as pe's new
// baseline (lastWritten plus the self-write fingerprint) so a watch
// event echoing this exact write back is recognized and suppressed. On
// failure the project is left dirty and, unless the Store is closing, a
// fresh idle timer is armed so the Store keeps retrying, per Store.
// Flush's doc — Close arms no such timer, since nothing will ever be
// there to catch its eventual SaveFailed. Must be called with s.mu held.
func (s *fileStore) writeAndRecordLocked(id store.ProjectID, pe *projectEntry, data []byte) error {
	if err := fsx.AtomicWrite(s.fs, pe.path, data); err != nil {
		pe.dirty = true
		if !s.closed {
			pe.idleTimer = time.AfterFunc(s.debounce, func() { s.post(func() { s.doFlush(id) }) })
		}
		return err
	}
	pe.lastWritten = data
	s.recordFingerprintLocked(pe, data)
	s.writeMetaLocked(pe)
	return nil
}

// flushAndCollect flushes id immediately — bypassing the debounce, for
// every mutation invariant 14's "Flush immediately" list names — and
// appends SaveFailed to events if that write failed, or whatever
// flushLocked itself produced (e.g. ConflictResolved) otherwise. Must be
// called with s.mu held.
func (s *fileStore) flushAndCollect(id store.ProjectID, events []store.Event) []store.Event {
	evs, err := s.flushLocked(id)
	if err != nil {
		return append(events, store.SaveFailed{})
	}
	return append(events, evs...)
}
