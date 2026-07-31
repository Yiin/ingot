package fsstore

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/mdfile"
)

// load reads every projects/*.md file into s.projects/s.order. Called
// once, from New, before the Store is handed to a caller.
func (s *fileStore) load() error {
	entries, err := s.fs.ReadDir(s.paths.Projects)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("fsstore: read projects dir: %w", err)
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".md") {
			continue
		}
		if err := s.loadProject(name); err != nil {
			return err
		}
	}

	// ReadDir already returns entries sorted by name on both fsx
	// implementations, but sort explicitly rather than depend on that.
	sort.Slice(s.order, func(i, j int) bool {
		return s.projects[s.order[i]].slug < s.projects[s.order[j]].slug
	})
	return nil
}

// loadProject reads and parses one project file. See
// parseIncomingProjectLocked for the read-only/id/empty-sections rules
// applied to its content.
func (s *fileStore) loadProject(name string) error {
	path := filepath.Join(s.paths.Projects, name)
	raw, err := s.fs.ReadFile(path)
	if err != nil {
		return fmt.Errorf("fsstore: read %s: %w", path, err)
	}

	proj, readOnly := s.parseIncomingProjectLocked(raw)

	slug := strings.TrimSuffix(name, ".md")
	pe := &projectEntry{
		proj:        proj,
		path:        path,
		slug:        slug,
		readOnly:    readOnly,
		lastWritten: raw,
	}
	s.recordFingerprintLocked(pe, raw)
	s.projects[proj.ID] = pe
	s.order = append(s.order, proj.ID)
	return nil
}

// parseIncomingProjectLocked parses raw into a Project and decides
// whether it's safe to write back, per invariant 14: any parse warning
// or a schema newer than mdfile understands puts it in read-only mode.
// A project with no persisted front-matter id (a hand-typed checklist
// file, per mdfile's grammar) — or whose id collides with one already
// known, most likely two files sharing a hand-copied front-matter block
// — gets a fresh one minted now, since the rest of the Store keys
// everything by ProjectID; an empty Sections list gets a lead section
// synthesized, since invariant 3 requires every project to have at
// least one. Shared by the startup loader and the watcher's
// external-create path (reload.go), which face the identical "a project
// file just appeared" problem. Must be called with s.mu held (loadProject
// calls it before New hands the Store to a caller, when no concurrent
// access is possible, which also satisfies this).
func (s *fileStore) parseIncomingProjectLocked(raw []byte) (store.Project, bool) {
	proj, warnings, parseErr := mdfile.Parse(raw)
	readOnly := parseErr != nil || len(warnings) > 0 || proj.Schema > mdfile.CurrentSchema

	if proj.ID == "" {
		proj.ID = store.ProjectID(s.newID())
	}
	if _, collision := s.projects[proj.ID]; collision {
		proj.ID = store.ProjectID(s.newID())
	}
	if len(proj.Sections) == 0 {
		proj.Sections = []store.Section{{ID: store.SectionID(s.newID())}}
	}
	return proj, readOnly
}
