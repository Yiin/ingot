package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"time"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsx"
)

// CurrentSchema is the sidecar schema version this package reads and
// writes. Load discards a sidecar whose schema is newer than this,
// rather than risk misinterpreting a shape a future version introduced.
const CurrentSchema = 1

// keyLen is how many hex characters of sha256(body) key an Entry — long
// enough that two distinct note bodies in one project collide only by
// astronomical chance, short enough to stay readable in a hand-inspected
// JSON file.
const keyLen = 16

// Entry is one note's advisory metadata.
type Entry struct {
	Created time.Time
	DoneAt  time.Time
	Source  store.Source
	App     string
	URI     string
}

// empty reports whether e carries nothing worth persisting: everything a
// note gets by default when parsed straight out of a Markdown file with
// no sidecar record at all.
func (e Entry) empty() bool {
	return e.Created.IsZero() && e.DoneAt.IsZero() && e.Source == store.SourceTyped && e.App == "" && e.URI == ""
}

// Key returns the sidecar key for a note body: the first keyLen hex
// characters of sha256(body). It is stable across reordering, section
// moves, and a Done toggle (which never touches Body), and intentionally
// changes on any body edit, since provenance describes a specific body.
func Key(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])[:keyLen]
}

// entryJSON is Entry's on-disk shape. Timestamp fields are pointers so a
// zero-value time.Time is omitted rather than written out as a
// misleading "0001-01-01T00:00:00Z".
type entryJSON struct {
	Created *time.Time `json:"created,omitempty"`
	DoneAt  *time.Time `json:"doneAt,omitempty"`
	Source  string     `json:"source,omitempty"`
	App     string     `json:"app,omitempty"`
	URI     string     `json:"uri,omitempty"`
}

var sourceNames = map[store.Source]string{
	store.SourceTyped:    "typed",
	store.SourceCaptured: "captured",
	store.SourceMerged:   "merged",
	store.SourceImported: "imported",
}

var sourceValues = map[string]store.Source{
	"typed":    store.SourceTyped,
	"captured": store.SourceCaptured,
	"merged":   store.SourceMerged,
	"imported": store.SourceImported,
}

func (e Entry) toJSON() entryJSON {
	out := entryJSON{App: e.App, URI: e.URI, Source: sourceNames[e.Source]}
	if !e.Created.IsZero() {
		t := e.Created.UTC()
		out.Created = &t
	}
	if !e.DoneAt.IsZero() {
		t := e.DoneAt.UTC()
		out.DoneAt = &t
	}
	return out
}

// toEntry converts back. An unrecognized or absent source name defaults
// to SourceTyped (sourceValues' zero value) rather than failing the
// whole entry over one unknown field.
func (j entryJSON) toEntry() Entry {
	e := Entry{App: j.App, URI: j.URI, Source: sourceValues[j.Source]}
	if j.Created != nil {
		e.Created = *j.Created
	}
	if j.DoneAt != nil {
		e.DoneAt = *j.DoneAt
	}
	return e
}

// sidecarFile is meta/<slug>.json's on-disk shape.
type sidecarFile struct {
	Schema  int                  `json:"schema"`
	Entries map[string]entryJSON `json:"entries"`
}

// Load reads path's sidecar and returns its entries keyed by Key. A
// missing file, invalid JSON, or a schema newer than CurrentSchema all
// degrade to a nil map rather than an error: an unrecognized or corrupt
// sidecar must never block the project it describes from loading, and
// losing provenance is the only acceptable cost.
func Load(fsys fsx.FS, path string) map[string]Entry {
	raw, err := fsys.ReadFile(path)
	if err != nil {
		return nil
	}
	var sf sidecarFile
	if err := json.Unmarshal(raw, &sf); err != nil {
		return nil
	}
	if sf.Schema > CurrentSchema {
		return nil
	}
	if len(sf.Entries) == 0 {
		return nil
	}
	out := make(map[string]Entry, len(sf.Entries))
	for k, v := range sf.Entries {
		out[k] = v.toEntry()
	}
	return out
}

// Apply restores Created, DoneAt, Source, App, and URI onto every note
// in sections whose body hashes to a key present in entries, mutating
// sections in place. A note with no matching entry is marked
// SourceImported: from the sidecar's perspective, a note with no record
// can only have arrived by a hand edit of the Markdown file, discovered
// on this reload rather than created through the app.
func Apply(sections []store.Section, entries map[string]Entry) {
	for si := range sections {
		notes := sections[si].Notes
		for ni := range notes {
			e, ok := entries[Key(notes[ni].Body)]
			if !ok {
				notes[ni].Source = store.SourceImported
				continue
			}
			notes[ni].Created = e.Created
			notes[ni].DoneAt = e.DoneAt
			notes[ni].Source = e.Source
			notes[ni].App = e.App
			notes[ni].URI = e.URI
		}
	}
}

// Extract builds the sidecar entries worth persisting for sections: one
// per note carrying any non-default Created, DoneAt, Source, App, or
// URI. A note with none of those is omitted — Apply already reconstructs
// it identically (SourceImported, everything else zero) with no entry at
// all, which is what lets Save delete the sidecar entirely once a
// project holds nothing but hand-typed, never-annotated notes.
func Extract(sections []store.Section) map[string]Entry {
	out := make(map[string]Entry)
	for _, sec := range sections {
		for _, n := range sec.Notes {
			e := Entry{Created: n.Created, DoneAt: n.DoneAt, Source: n.Source, App: n.App, URI: n.URI}
			if e.empty() {
				continue
			}
			out[Key(n.Body)] = e
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Save writes entries to path, or removes path entirely when entries is
// empty — an empty sidecar is never left on disk, per the "delete it
// when empty" contract.
func Save(fsys fsx.FS, path string, entries map[string]Entry) error {
	if len(entries) == 0 {
		if err := fsys.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}

	sf := sidecarFile{Schema: CurrentSchema, Entries: make(map[string]entryJSON, len(entries))}
	for k, v := range entries {
		sf.Entries[k] = v.toJSON()
	}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	return fsx.AtomicWrite(fsys, path, data)
}
