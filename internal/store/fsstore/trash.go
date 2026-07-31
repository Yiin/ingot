package fsstore

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"time"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/mdfile"
)

// trashMaxAge and trashMaxFiles bound how long pruneTrash lets trash/
// grow, whichever binds first.
const (
	trashMaxAge   = 30 * 24 * time.Hour
	trashMaxFiles = 200
)

// trashSection is one Section's worth of removed notes, the unit
// writeTrashLocked renders into a trash file.
type trashSection struct {
	title string
	notes []store.Note
}

// trashTimestamp formats t as a compact, filename-safe RFC3339: no ':'
// or '-' separators, since those would collide with the "-" this
// package's naming convention uses to join its own fields.
func trashTimestamp(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// trashPathLocked returns the next free path in Trash named
// "<timestamp>-<slug>-<op>.md", numbering past a collision (two
// operations against the same project in the same wall-clock second —
// realistic under a frozen test clock, or a fast double action) rather
// than silently overwriting an earlier trash file. Returns "" if Trash
// isn't configured. Must be called with s.mu held.
func (s *fileStore) trashPathLocked(slug, op string) string {
	if s.paths.Trash == "" {
		return ""
	}
	base := fmt.Sprintf("%s-%s-%s", trashTimestamp(s.now()), slug, op)
	name := base + ".md"
	for n := 2; s.trashNameExistsLocked(name); n++ {
		name = fmt.Sprintf("%s-%d.md", base, n)
	}
	return filepath.Join(s.paths.Trash, name)
}

func (s *fileStore) trashNameExistsLocked(name string) bool {
	_, err := s.fs.Stat(filepath.Join(s.paths.Trash, name))
	return err == nil
}

// writeTrashLocked renders sections into the same Markdown grammar
// mdfile reads and writes, with front matter recording which project and
// operation produced it, and writes it to one new file under Trash. The
// result mdfile.Parses straight back into the removed notes, satisfying
// invariant 14's "each destructive op writes exactly one trash file that
// re-parses to the removed notes." Returns "", nil if Trash isn't
// configured or pid names no known project. Must be called with s.mu
// held.
func (s *fileStore) writeTrashLocked(pid store.ProjectID, op string, sections []trashSection) (string, error) {
	pe := s.projects[pid]
	if pe == nil {
		return "", nil
	}
	path := s.trashPathLocked(pe.slug, op)
	if path == "" {
		return "", nil
	}

	now := s.now()
	proj := store.Project{
		Schema:  mdfile.CurrentSchema,
		Created: now,
		Extra: map[string]string{
			"project":   string(pid),
			"op":        op,
			"timestamp": now.UTC().Format(time.RFC3339),
		},
	}
	for _, sec := range sections {
		proj.Sections = append(proj.Sections, store.Section{Title: sec.title, Notes: sec.notes})
	}
	data, err := mdfile.Format(proj)
	if err != nil {
		return "", err
	}

	if err := s.fs.MkdirAll(s.paths.Trash, 0o755); err != nil {
		return "", err
	}
	if err := fsx.AtomicWrite(s.fs, path, data); err != nil {
		return "", err
	}
	return path, nil
}

// writeDestructiveTrashLocked groups removed by the project each note
// came from — removed is already in document order per
// snapshotRemovedLocked, so each project's entries are contiguous — and
// writes one trash file per project touched. Callers pass this the
// pre-mutation snapshot for DeleteNotes/ClearDone; a project that
// contributed no entries gets no file, matching invariant 14 exactly:
// one trash file per destructive op per project it actually touched.
// Must be called with s.mu held.
func (s *fileStore) writeDestructiveTrashLocked(op string, removed []removedNote) []store.Event {
	if len(removed) == 0 {
		return nil
	}
	var events []store.Event
	for _, pid := range s.order {
		var sections []trashSection
		var lastSection store.SectionID
		have := false
		for _, rn := range removed {
			if rn.project != pid {
				continue
			}
			if !have || rn.section != lastSection {
				sections = append(sections, trashSection{title: rn.title})
				lastSection = rn.section
				have = true
			}
			last := &sections[len(sections)-1]
			last.notes = append(last.notes, rn.note)
		}
		if len(sections) == 0 {
			continue
		}
		if _, err := s.writeTrashLocked(pid, op, sections); err != nil {
			events = append(events, store.SaveFailed{})
		}
	}
	return events
}

// writeExternalTrashLocked copies raw — the on-disk content a conflict
// is about to overwrite with the panel's in-memory version — verbatim to
// Trash, so an external edit that lost a write race is still recoverable
// as a plain file the user can open in an editor. Must be called with
// s.mu held.
func (s *fileStore) writeExternalTrashLocked(slug string, raw []byte) (string, error) {
	path := s.trashPathLocked(slug, "external")
	if path == "" {
		return "", nil
	}
	if err := s.fs.MkdirAll(s.paths.Trash, 0o755); err != nil {
		return "", err
	}
	if err := fsx.AtomicWrite(s.fs, path, raw); err != nil {
		return "", err
	}
	return path, nil
}

// moveToTrashLocked renames pe's file into Trash instead of unlinking
// it, so DeleteProject leaves a byte-exact recovery copy behind — "no
// data is lost either way" extends to deleting a whole project, not just
// notes within one. Returns "", nil if Trash isn't configured or there's
// nothing on disk to move (a project created but never flushed). Must be
// called with s.mu held.
func (s *fileStore) moveToTrashLocked(pe *projectEntry, op string) (string, error) {
	if s.paths.Trash == "" {
		return "", nil
	}
	dst := s.trashPathLocked(pe.slug, op)
	if err := s.fs.MkdirAll(s.paths.Trash, 0o755); err != nil {
		return "", err
	}
	if err := s.fs.Rename(pe.path, dst); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	if err := s.fs.SyncDir(s.paths.Trash); err != nil {
		return "", err
	}
	return dst, nil
}

// writeProjectSnapshotTrashLocked formats pe's current in-memory content
// and writes it to a new trash file. Unlike moveToTrashLocked, there's
// no on-disk file left to move — this is for the case where one
// vanished out from under a project that still had unsaved edits, so
// the last thing worth preserving is what's in memory, not what used to
// be on disk. Must be called with s.mu held.
func (s *fileStore) writeProjectSnapshotTrashLocked(pe *projectEntry, op string) (string, error) {
	path := s.trashPathLocked(pe.slug, op)
	if path == "" {
		return "", nil
	}
	data, err := mdfile.Format(pe.proj)
	if err != nil {
		return "", err
	}
	if err := s.fs.MkdirAll(s.paths.Trash, 0o755); err != nil {
		return "", err
	}
	if err := fsx.AtomicWrite(s.fs, path, data); err != nil {
		return "", err
	}
	return path, nil
}

// pruneTrash removes the oldest files under Trash past trashMaxAge or
// trashMaxFiles, whichever binds first. Called once from New, before the
// Store is handed to a caller; best-effort, since a dirty trash
// directory is hygiene, not correctness, and shouldn't fail startup.
func (s *fileStore) pruneTrash() {
	if s.paths.Trash == "" {
		return
	}
	entries, err := s.fs.ReadDir(s.paths.Trash)
	if err != nil {
		return
	}

	type file struct {
		name    string
		modTime time.Time
	}
	files := make([]file, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, file{name: e.Name(), modTime: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.After(files[j].modTime) })

	cutoff := s.now().Add(-trashMaxAge)
	for i, f := range files {
		if i < trashMaxFiles && !f.modTime.Before(cutoff) {
			continue
		}
		_ = s.fs.Remove(filepath.Join(s.paths.Trash, f.name))
	}
}
