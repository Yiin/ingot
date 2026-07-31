package fsstore

import (
	"context"
	"testing"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/paths"
)

// metaLayout mirrors testLayout but also configures a Meta directory,
// since testLayout deliberately leaves it unset so every pre-existing
// fsstore test exercises the "no sidecar layer configured" path.
func metaLayout() paths.Layout {
	l := testLayout()
	l.Meta = "/data/meta"
	return l
}

// --- provenance sidecar (copper-l2z.35) --------------------------------

func TestProvenanceSurvivesReload(t *testing.T) {
	mem := fsx.NewMem()
	clock := newFakeClock()
	s, err := New(Options{FS: mem, Paths: metaLayout(), Now: clock.Now, NewID: seqIDs()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	mustCreateProject(t, s, "Work")

	nid, err := s.AppendToDefault("captured from the web", store.Origin{App: "Firefox", URI: "https://example.com"})
	if err != nil {
		t.Fatalf("AppendToDefault: %v", err)
	}
	if err := s.SetNoteDone(nid, true); err != nil {
		t.Fatalf("SetNoteDone: %v", err)
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// A brand new Store instance over the same MemFS simulates a
	// restart: mdfile's own format only round-trips Body and Done, so
	// Created/DoneAt/Source/App/URI can only survive via the sidecar.
	reloaded, err := New(Options{FS: mem, Paths: metaLayout(), Now: clock.Now, NewID: seqIDs()})
	if err != nil {
		t.Fatalf("New (reload): %v", err)
	}

	var found store.Note
	ok := false
	for _, ref := range reloaded.Projects() {
		if ref.Title != "Work" {
			continue
		}
		proj, err := reloaded.Project(ref.ID)
		if err != nil {
			t.Fatalf("Project: %v", err)
		}
		for _, sec := range proj.Sections {
			for _, n := range sec.Notes {
				if n.Body == "captured from the web" {
					found, ok = n, true
				}
			}
		}
	}
	if !ok {
		t.Fatalf("note not found after reload")
	}
	if found.Source != store.SourceCaptured {
		t.Errorf("Source = %v, want SourceCaptured", found.Source)
	}
	if found.App != "Firefox" || found.URI != "https://example.com" {
		t.Errorf("App/URI = %q/%q, want Firefox/https://example.com", found.App, found.URI)
	}
	if !found.Done {
		t.Errorf("Done = false, want true")
	}
	if found.Created.IsZero() {
		t.Errorf("Created is zero, want restored from sidecar")
	}
	if found.DoneAt.IsZero() {
		t.Errorf("DoneAt is zero, want restored from sidecar")
	}
}

func TestProjectLoadsWithSidecarAbsentEmptyMalformedOrUnknownSchema(t *testing.T) {
	cases := []struct {
		name string
		meta string // "" means no meta file at all
	}{
		{"absent", ""},
		{"empty", ""},
		{"malformed", "{not json"},
		{"unknown schema", `{"schema":999,"entries":{"x":{"app":"Y"}}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mem := fsx.NewMem()
			seedFile(t, mem, "work.md", "---\ningot: 1\nid: 1111111111111111\ntitle: Work\n---\n\n- [ ] hand-typed note\n")
			if tc.name == "empty" {
				_ = mem.MkdirAll("/data/meta", 0o755)
				writeRaw(t, mem, "/data/meta/work.json", nil)
			} else if tc.meta != "" {
				_ = mem.MkdirAll("/data/meta", 0o755)
				writeRaw(t, mem, "/data/meta/work.json", []byte(tc.meta))
			}

			s, err := New(Options{FS: mem, Paths: metaLayout()})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			proj, err := s.Project(store.ProjectID("1111111111111111"))
			if err != nil {
				t.Fatalf("Project: %v", err)
			}
			if len(proj.Sections) != 1 || len(proj.Sections[0].Notes) != 1 {
				t.Fatalf("project did not load its note: %+v", proj)
			}
			note := proj.Sections[0].Notes[0]
			if note.Body != "hand-typed note" {
				t.Errorf("Body = %q, want %q", note.Body, "hand-typed note")
			}
			// Losing only metadata: the note itself loads, and with no
			// sidecar record it is honestly reported as SourceImported
			// rather than defaulting to SourceTyped.
			if note.Source != store.SourceImported {
				t.Errorf("Source = %v, want SourceImported", note.Source)
			}
		})
	}
}

func TestDeleteProjectRemovesSidecar(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, func(o *Options) { o.Paths = metaLayout() })
	pid := mustCreateProject(t, s, "Gone")
	sec := firstSection(t, s, pid)
	if _, err := s.AppendToDefault("captured note", store.Origin{App: "X"}); err != nil {
		t.Fatalf("AppendToDefault: %v", err)
	}
	_ = sec
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if _, err := mem.ReadFile("/data/meta/gone.json"); err != nil {
		t.Fatalf("sidecar not written before delete: %v", err)
	}

	if err := s.DeleteProject(pid); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := mem.ReadFile("/data/meta/gone.json"); err == nil {
		t.Errorf("sidecar still present after DeleteProject")
	}
}

func TestNoSidecarWrittenForPurelyHandTypedProject(t *testing.T) {
	mem := fsx.NewMem()
	s := newStore(t, mem, func(o *Options) { o.Paths = metaLayout() })
	pid := mustCreateProject(t, s, "Plain")
	sec := firstSection(t, s, pid)
	if _, err := s.AddNote(sec, "typed note"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if err := s.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// A typed note does carry a Created timestamp (set at AddNote time),
	// so its sidecar entry is not empty — the file is written.
	if _, err := mem.ReadFile("/data/meta/plain.json"); err != nil {
		t.Fatalf("expected sidecar for a note with a Created timestamp: %v", err)
	}
}

func writeRaw(t *testing.T, fsys fsx.FS, path string, data []byte) {
	t.Helper()
	f, err := fsys.Create(path)
	if err != nil {
		t.Fatalf("Create(%s): %v", path, err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("Write(%s): %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close(%s): %v", path, err)
	}
}
