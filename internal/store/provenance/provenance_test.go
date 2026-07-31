package provenance

import (
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsx"
)

func TestKeyStableAcrossCalls(t *testing.T) {
	a := Key("buy milk")
	b := Key("buy milk")
	if a != b {
		t.Fatalf("Key not stable: %q vs %q", a, b)
	}
	if len(a) != keyLen {
		t.Fatalf("Key length = %d, want %d", len(a), keyLen)
	}
	if c := Key("buy oat milk"); c == a {
		t.Fatalf("Key collided for distinct bodies")
	}
}

func TestLoadMissingFile(t *testing.T) {
	mem := fsx.NewMem()
	if got := Load(mem, "/meta/none.json"); got != nil {
		t.Fatalf("Load(missing) = %#v, want nil", got)
	}
}

func TestLoadMalformedJSON(t *testing.T) {
	mem := fsx.NewMem()
	_ = mem.MkdirAll("/meta", 0o755)
	writeRaw(t, mem, "/meta/p.json", []byte("{not json"))
	if got := Load(mem, "/meta/p.json"); got != nil {
		t.Fatalf("Load(malformed) = %#v, want nil", got)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	mem := fsx.NewMem()
	_ = mem.MkdirAll("/meta", 0o755)
	writeRaw(t, mem, "/meta/p.json", []byte(""))
	if got := Load(mem, "/meta/p.json"); got != nil {
		t.Fatalf("Load(empty) = %#v, want nil", got)
	}
}

func TestLoadNewerSchemaDiscarded(t *testing.T) {
	mem := fsx.NewMem()
	_ = mem.MkdirAll("/meta", 0o755)
	writeRaw(t, mem, "/meta/p.json", []byte(`{"schema":999,"entries":{"abc":{"app":"Ghostty"}}}`))
	if got := Load(mem, "/meta/p.json"); got != nil {
		t.Fatalf("Load(newer schema) = %#v, want nil", got)
	}
}

func TestLoadUnknownSourceNameDefaultsRatherThanFails(t *testing.T) {
	mem := fsx.NewMem()
	_ = mem.MkdirAll("/meta", 0o755)
	writeRaw(t, mem, "/meta/p.json", []byte(`{"schema":1,"entries":{"abc":{"source":"from-the-future"}}}`))
	got := Load(mem, "/meta/p.json")
	if got == nil {
		t.Fatalf("Load discarded the whole sidecar over one unknown source name")
	}
	if got["abc"].Source != store.SourceTyped {
		t.Fatalf("unknown source name = %v, want SourceTyped default", got["abc"].Source)
	}
}

func TestSaveThenLoadRoundTrip(t *testing.T) {
	mem := fsx.NewMem()
	_ = mem.MkdirAll("/meta", 0o755)
	created := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	doneAt := time.Date(2026, 3, 2, 11, 0, 0, 0, time.UTC)
	entries := map[string]Entry{
		Key("buy milk"): {Created: created, DoneAt: doneAt, Source: store.SourceCaptured, App: "Firefox", URI: "https://example.com"},
	}

	if err := Save(mem, "/meta/p.json", entries); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got := Load(mem, "/meta/p.json")
	entry, ok := got[Key("buy milk")]
	if !ok {
		t.Fatalf("Load did not return the saved entry")
	}
	if !entry.Created.Equal(created) || !entry.DoneAt.Equal(doneAt) {
		t.Fatalf("timestamps not round-tripped: got %+v", entry)
	}
	if entry.Source != store.SourceCaptured || entry.App != "Firefox" || entry.URI != "https://example.com" {
		t.Fatalf("fields not round-tripped: got %+v", entry)
	}
}

func TestSaveEmptyDeletesFile(t *testing.T) {
	mem := fsx.NewMem()
	_ = mem.MkdirAll("/meta", 0o755)
	if err := Save(mem, "/meta/p.json", map[string]Entry{"k": {App: "X"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := mem.ReadFile("/meta/p.json"); err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}

	if err := Save(mem, "/meta/p.json", nil); err != nil {
		t.Fatalf("Save(empty): %v", err)
	}
	if _, err := mem.ReadFile("/meta/p.json"); err == nil {
		t.Fatalf("sidecar still present after emptying")
	}
}

func TestSaveEmptyOnAbsentFileIsNoop(t *testing.T) {
	mem := fsx.NewMem()
	_ = mem.MkdirAll("/meta", 0o755)
	if err := Save(mem, "/meta/never-existed.json", nil); err != nil {
		t.Fatalf("Save(empty, absent): %v", err)
	}
}

func TestApplyRestoresMatchedNotesAndMarksUnmatchedImported(t *testing.T) {
	created := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	sections := []store.Section{
		{Notes: []store.Note{
			{Body: "buy milk"},
			{Body: "hand-typed, no record"},
		}},
	}
	entries := map[string]Entry{
		Key("buy milk"): {Created: created, Source: store.SourceCaptured, App: "Firefox"},
	}

	Apply(sections, entries)

	matched := sections[0].Notes[0]
	if matched.Source != store.SourceCaptured || matched.App != "Firefox" || !matched.Created.Equal(created) {
		t.Fatalf("matched note not restored: %+v", matched)
	}
	unmatched := sections[0].Notes[1]
	if unmatched.Source != store.SourceImported {
		t.Fatalf("unmatched note Source = %v, want SourceImported", unmatched.Source)
	}
	if !unmatched.Created.IsZero() {
		t.Fatalf("unmatched note Created = %v, want zero", unmatched.Created)
	}
}

func TestExtractOmitsNotesWithNoMetadata(t *testing.T) {
	sections := []store.Section{
		{Notes: []store.Note{
			{Body: "plain", Source: store.SourceTyped},
			{Body: "captured", Source: store.SourceCaptured, App: "Firefox"},
		}},
	}

	got := Extract(sections)
	if _, ok := got[Key("plain")]; ok {
		t.Fatalf("Extract kept a note with no metadata worth persisting")
	}
	if _, ok := got[Key("captured")]; !ok {
		t.Fatalf("Extract dropped a note carrying metadata")
	}
}

func TestExtractAllDefaultReturnsNil(t *testing.T) {
	sections := []store.Section{
		{Notes: []store.Note{{Body: "plain", Source: store.SourceTyped}}},
	}
	if got := Extract(sections); got != nil {
		t.Fatalf("Extract = %#v, want nil when nothing carries metadata", got)
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
