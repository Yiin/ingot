package mdfile_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/mdfile"
)

func TestCanonicalRoundTrip(t *testing.T) {
	files, err := filepath.Glob("testdata/canonical/*.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no canonical testdata files found")
	}
	for _, path := range files {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			proj, warnings, err := mdfile.Parse(want)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(warnings) != 0 {
				t.Errorf("unexpected warnings: %v", warnings)
			}
			got, err := mdfile.Format(proj)
			if err != nil {
				t.Fatalf("Format: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("Format(Parse(f)) != f\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func TestMessyNormalizes(t *testing.T) {
	files, err := filepath.Glob("testdata/messy/*.in.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no messy testdata files found")
	}
	for _, inPath := range files {
		inPath := inPath
		name := strings.TrimSuffix(filepath.Base(inPath), ".in.md")
		t.Run(name, func(t *testing.T) {
			in, err := os.ReadFile(inPath)
			if err != nil {
				t.Fatal(err)
			}
			wantPath := filepath.Join(filepath.Dir(inPath), name+".want.md")
			want, err := os.ReadFile(wantPath)
			if err != nil {
				t.Fatal(err)
			}
			proj, _, err := mdfile.Parse(in)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			got, err := mdfile.Format(proj)
			if err != nil {
				t.Fatalf("Format: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("normalized output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func TestFormatSchemaTooNew(t *testing.T) {
	_, err := mdfile.Format(store.Project{Schema: 2})
	if !errors.Is(err, mdfile.ErrSchemaTooNew) {
		t.Fatalf("Format with Schema=2: got err %v, want ErrSchemaTooNew", err)
	}
}

func TestFormatSchemaCurrentOK(t *testing.T) {
	if _, err := mdfile.Format(store.Project{Schema: mdfile.CurrentSchema}); err != nil {
		t.Fatalf("Format with Schema=CurrentSchema: %v", err)
	}
}

func TestPropertyRoundTrip(t *testing.T) {
	fixtures := []store.Project{
		{},
		{
			Sections: []store.Section{
				{Title: "", Notes: []store.Note{{Body: "a lone note"}}},
			},
		},
		{
			ID:      store.ProjectID("aaaaaaaaaaaaaaaa"),
			Title:   "Work",
			Created: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
			Schema:  1,
			Extra:   map[string]string{"z": "last", "a": "first"},
			Sections: []store.Section{
				{Title: "", Notes: []store.Note{
					{Body: "line one\n\nline three", Done: true},
				}},
				{Title: "Later", Notes: []store.Note{
					{Body: "```\ncode\n```"},
					{Body: "## looks like a heading but is not"},
				}},
			},
		},
		{
			ID:    store.ProjectID("bbbbbbbbbbbbbbbb"),
			Title: "",
			Sections: []store.Section{
				{Title: "Dup", Notes: []store.Note{{Body: "one"}}},
				{Title: "Dup", Notes: []store.Note{{Body: "two"}}},
				{Title: "Empty"},
			},
		},
		// A project with a Title but no ID/Created yet — front matter
		// must not carry empty "id: "/"created: " lines that Parse
		// would then reject back into Extra.
		{
			Title:  "X",
			Schema: 1,
		},
		// A lone \r (not part of \r\n) in a note body must not survive
		// as a raw byte, or it silently changes shape on the next load.
		{
			Sections: []store.Section{
				{Title: "", Notes: []store.Note{{Body: "foo\rbar"}}},
			},
		},
	}

	for i, p := range fixtures {
		once, err := mdfile.Format(p)
		if err != nil {
			t.Fatalf("fixture %d: Format: %v", i, err)
		}
		reparsed, _, err := mdfile.Parse(once)
		if err != nil {
			t.Fatalf("fixture %d: Parse: %v", i, err)
		}
		twice, err := mdfile.Format(reparsed)
		if err != nil {
			t.Fatalf("fixture %d: Format (second): %v", i, err)
		}
		if !bytes.Equal(once, twice) {
			t.Errorf("fixture %d: Format(Parse(Format(p))) != Format(p)\n--- once ---\n%s\n--- twice ---\n%s", i, once, twice)
		}
	}
}

func TestUnterminatedFrontMatterWarns(t *testing.T) {
	_, warnings, err := mdfile.Parse([]byte("---\nnot closed\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %v", len(warnings), warnings)
	}
}

func TestMalformedFrontMatterLineWarns(t *testing.T) {
	_, warnings, err := mdfile.Parse([]byte("---\nno colon here\ntitle: ok\n---\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %v", len(warnings), warnings)
	}
}

func TestInvalidCreatedFallsBackToExtra(t *testing.T) {
	p, warnings, err := mdfile.Parse([]byte("---\ningot: 1\nid: 0123456789abcdef\ntitle: X\ncreated: not-a-time\n---\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %v", len(warnings), warnings)
	}
	if !p.Created.IsZero() {
		t.Errorf("Created = %v, want zero", p.Created)
	}
	if p.Extra["created"] != "not-a-time" {
		t.Errorf("Extra[created] = %q, want %q", p.Extra["created"], "not-a-time")
	}
}

func FuzzParse(f *testing.F) {
	seeds, _ := filepath.Glob("testdata/canonical/*.md")
	more, _ := filepath.Glob("testdata/messy/*.md")
	seeds = append(seeds, more...)
	for _, path := range seeds {
		b, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	f.Add([]byte(""))
	f.Add([]byte("---\n---\n"))
	f.Add([]byte("---\nunterminated"))

	f.Fuzz(func(t *testing.T, b []byte) {
		proj, _, err := mdfile.Parse(b)
		if err != nil {
			return
		}
		if proj.Schema > mdfile.CurrentSchema {
			return
		}
		if _, err := mdfile.Format(proj); err != nil {
			t.Fatalf("Format of a just-parsed project failed: %v", err)
		}
	})
}
