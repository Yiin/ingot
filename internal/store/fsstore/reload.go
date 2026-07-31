package fsstore

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/mdfile"
)

// recordFingerprintLocked captures path's current on-disk (size, mtime,
// sha256(raw)) onto pe, so a future watch event reporting exactly this
// state is recognized as not a change — whether raw got there via this
// Store's own AtomicWrite, the initial load, or a reload/conflict that
// just adopted a version as the new baseline. Must be called with s.mu
// held.
func (s *fileStore) recordFingerprintLocked(pe *projectEntry, raw []byte) {
	pe.selfSHA = sha256.Sum256(raw)
	if info, err := s.fs.Stat(pe.path); err == nil {
		pe.selfSize = info.Size()
		pe.selfMTime = info.ModTime()
	}
}

// isOwnWrite reports whether info/raw — path's current on-disk state —
// matches the fingerprint recordFingerprintLocked most recently captured
// for pe, i.e. this event is the Store's own write echoing back through
// the watcher rather than a change to react to.
func isOwnWrite(pe *projectEntry, info fs.FileInfo, raw []byte) bool {
	return info.Size() == pe.selfSize &&
		info.ModTime().Equal(pe.selfMTime) &&
		sha256.Sum256(raw) == pe.selfSHA
}

// detectExternalChangeLocked Stats and, if needed, reads pe.path to
// decide whether it currently holds something this Store didn't put
// there — the same isOwnWrite check handleWatchPathLocked uses, run
// directly on flushLocked's write path instead of waiting for the
// watcher's 200ms debounce to settle on the same fsnotify event. A
// mismatch is copied to Trash immediately, exactly as
// resolveConflictLocked does for a watcher-detected conflict, so
// flushLocked's own write can safely overwrite the live file without
// losing the external edit. conflict is false, with trashPath == "",
// when there's nothing to reconcile: the file doesn't exist yet (a
// project not yet flushed for the first time — or an external delete,
// which the watcher's own handleExternalRemoveLocked path owns), or its
// live content still matches this Store's last known fingerprint. Must
// be called with s.mu held.
func (s *fileStore) detectExternalChangeLocked(pe *projectEntry) (trashPath string, conflict bool, err error) {
	info, statErr := s.fs.Stat(pe.path)
	if statErr != nil {
		return "", false, nil
	}
	raw, err := s.fs.ReadFile(pe.path)
	if err != nil {
		return "", false, err
	}
	if isOwnWrite(pe, info, raw) {
		return "", false, nil
	}
	trashPath, err = s.writeExternalTrashLocked(pe.slug, raw)
	if err != nil {
		return "", false, err
	}
	return trashPath, true, nil
}

func isProjectMDPath(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, ".md") && !strings.HasPrefix(base, ".")
}

// projectByPathLocked finds the project whose file lives at path, if
// any. Must be called with s.mu held.
func (s *fileStore) projectByPathLocked(path string) (store.ProjectID, *projectEntry) {
	for _, pid := range s.order {
		if pe := s.projects[pid]; pe.path == path {
			return pid, pe
		}
	}
	return "", nil
}

// handleWatchPathLocked applies invariant 14's conflict policy to one
// settled path: Stat it to find out what actually happened (rather than
// trust any single fsnotify Op, which a debounced burst of Create/
// Write/Rename/Remove for one logical edit makes unreliable on its own),
// then dispatch to the matching case. Must be called with s.mu held.
func (s *fileStore) handleWatchPathLocked(path string) []store.Event {
	if !isProjectMDPath(path) {
		return nil
	}
	pid, pe := s.projectByPathLocked(path)

	info, statErr := s.fs.Stat(path)
	if statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			if pe == nil {
				return nil
			}
			return s.handleExternalRemoveLocked(pid, pe)
		}
		// A transient Stat failure: leave state as-is, a later event
		// (this path settling again, or another burst) will retry.
		return nil
	}

	raw, err := s.fs.ReadFile(path)
	if err != nil {
		return nil
	}

	if pe != nil && isOwnWrite(pe, info, raw) {
		return nil
	}
	if pe == nil {
		return s.handleExternalCreateLocked(path, raw)
	}
	if pe.dirty {
		return s.resolveConflictLocked(pid, pe, raw)
	}
	return s.reloadProjectLocked(pid, pe, raw)
}

// handleExternalRemoveLocked handles a tracked project's file vanishing
// out from under it (an external delete, or a sync tool's move-away). If
// pe had unsaved edits, they'd otherwise disappear with no trace the
// moment the in-memory entry is dropped — the same "no data lost either
// way" the pending-changes conflict branch promises for an overwrite
// applies here too, so a snapshot of the current in-memory content goes
// to Trash first. Must be called with s.mu held.
func (s *fileStore) handleExternalRemoveLocked(pid store.ProjectID, pe *projectEntry) []store.Event {
	var events []store.Event
	if pe.dirty {
		if _, err := s.writeProjectSnapshotTrashLocked(pe, "external-remove"); err != nil {
			events = append(events, store.SaveFailed{})
		}
	}
	return append(events, s.unregisterProjectLocked(pid)...)
}

// reloadProjectLocked handles invariant 14's "no pending in-memory
// changes" branch: reparse raw and adopt it wholesale, carrying runtime
// ids across by matching (section index, body) so a subscriber loses at
// worst a selection highlight, never data. Must be called with s.mu
// held.
func (s *fileStore) reloadProjectLocked(pid store.ProjectID, pe *projectEntry, raw []byte) []store.Event {
	newProj, warnings, parseErr := mdfile.Parse(raw)
	readOnly := parseErr != nil || len(warnings) > 0 || newProj.Schema > mdfile.CurrentSchema

	newProj.ID = pid
	if len(newProj.Sections) == 0 {
		newProj.Sections = []store.Section{{ID: store.SectionID(s.newID())}}
	}
	carryRuntimeIDs(&newProj, pe.proj)

	pe.proj = newProj
	pe.readOnly = readOnly
	pe.lastWritten = raw
	pe.dirty = false
	s.recordFingerprintLocked(pe, raw)
	s.clearUndoLocked()

	events := []store.Event{store.ProjectReloaded{}}
	if readOnly {
		events = append(events, store.ProjectReadOnly{})
	}
	return events
}

// carryRuntimeIDs mutates newProj's Sections in place: SectionIDs carry
// across from old by position, and each Note's ID plus the runtime-only
// fields mdfile.Parse never recovers (Created, Source, App, URI) carry
// across by matching (section index, body) against the first
// not-yet-matched old note with that body in the same section. A body
// that doesn't match anything just mints a fresh id and looks like a new
// note — the "lost selection highlight, never lost data" heuristic
// invariant 14 promises.
func carryRuntimeIDs(newProj *store.Project, old store.Project) {
	for i := range newProj.Sections {
		if i >= len(old.Sections) {
			break
		}
		oldSec := old.Sections[i]
		newProj.Sections[i].ID = oldSec.ID

		used := make([]bool, len(oldSec.Notes))
		for ni := range newProj.Sections[i].Notes {
			newNote := &newProj.Sections[i].Notes[ni]
			for oi := range oldSec.Notes {
				if used[oi] || oldSec.Notes[oi].Body != newNote.Body {
					continue
				}
				on := oldSec.Notes[oi]
				newNote.ID = on.ID
				newNote.Created = on.Created
				newNote.Source = on.Source
				newNote.App = on.App
				newNote.URI = on.URI
				used[oi] = true
				break
			}
		}
	}
}

// resolveConflictLocked handles invariant 14's "pending changes" branch:
// the panel wins. externalRaw is copied to Trash first so it's never
// lost, then the in-memory version is flushed over the live path. If the
// trash copy fails, the live file is left exactly as the external writer
// left it and pe stays dirty, so a plain retry of this same flow — not a
// silent overwrite with no backup — is what happens next. Must be called
// with s.mu held.
func (s *fileStore) resolveConflictLocked(pid store.ProjectID, pe *projectEntry, externalRaw []byte) []store.Event {
	trashPath, err := s.writeExternalTrashLocked(pe.slug, externalRaw)
	if err != nil {
		return []store.Event{store.SaveFailed{}}
	}
	// flushForceLocked, not flushLocked: pe.lastWritten still describes
	// what this Store wrote before the external overwrite, so an
	// in-memory version that happens to format identically to it would
	// make flushLocked's "unchanged" short-circuit skip the write
	// outright — leaving the external content live on disk right after
	// claiming the panel won.
	if err := s.flushForceLocked(pid); err != nil {
		return []store.Event{store.SaveFailed{}}
	}
	return []store.Event{store.ConflictResolved{ID: pid, SavedTo: trashPath}}
}

// handleExternalCreateLocked handles a *.md file appearing under
// Paths.Projects that the Store didn't put there — a hand-copied file, a
// sync tool, another instance's write. It mirrors loadProject's
// treatment of the same file if this had been startup instead of a live
// event. Must be called with s.mu held.
func (s *fileStore) handleExternalCreateLocked(path string, raw []byte) []store.Event {
	proj, readOnly := s.parseIncomingProjectLocked(raw)

	slug := strings.TrimSuffix(filepath.Base(path), ".md")
	pe := &projectEntry{proj: proj, path: path, slug: slug, readOnly: readOnly, lastWritten: raw}
	s.recordFingerprintLocked(pe, raw)

	s.projects[proj.ID] = pe
	s.order = append(s.order, proj.ID)
	sort.Slice(s.order, func(i, j int) bool {
		return s.projects[s.order[i]].slug < s.projects[s.order[j]].slug
	})
	s.clearUndoLocked()

	events := []store.Event{store.ProjectListChanged{}}
	if s.active == "" {
		s.active = proj.ID
		events = append(events, store.ActiveProjectChanged{})
		if err := s.writeStateLocked(); err != nil {
			events = append(events, store.SaveFailed{})
		}
	}
	if readOnly {
		events = append(events, store.ProjectReadOnly{})
	}
	return events
}
