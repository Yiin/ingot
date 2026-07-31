package paths

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/store/fsx"
)

func TestSweepTemp(t *testing.T) {
	l := Layout{Projects: "/data/ingot/projects"}
	mem := fsx.NewMem()
	if err := mem.MkdirAll(l.Projects, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	write := func(name string) {
		f, err := mem.Create(filepath.Join(l.Projects, name))
		if err != nil {
			t.Fatalf("Create(%s): %v", name, err)
		}
		if _, err := f.Write([]byte("x")); err != nil {
			t.Fatalf("Write(%s): %v", name, err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("Close(%s): %v", name, err)
		}
	}

	write("home-garden.md")             // a real project file
	write(".home-garden.md.tmp-abc123") // a stale temp file
	write(".foo.md.tmp-def456")         // another stale temp file
	write(".not-a-tmp-file")            // dotfile, but not the tmp pattern
	// ProjectFile now refuses to construct a name like this (validateSlug
	// rejects any dot), but SweepTemp is checked independently: a real
	// file that merely contains the substring ".tmp-" followed by
	// something other than pure hex must never be swept as if it were an
	// AtomicWrite leftover.
	write(".foo.tmp-1.md")

	if err := SweepTemp(mem, l, 0); err != nil {
		t.Fatalf("SweepTemp() error = %v", err)
	}

	entries, err := mem.ReadDir(l.Projects)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.Name()] = true
	}

	for _, want := range []string{"home-garden.md", ".not-a-tmp-file", ".foo.tmp-1.md"} {
		if !names[want] {
			t.Errorf("SweepTemp removed %q, want it kept", want)
		}
	}
	for _, gone := range []string{".home-garden.md.tmp-abc123", ".foo.md.tmp-def456"} {
		if names[gone] {
			t.Errorf("SweepTemp kept %q, want it removed", gone)
		}
	}
}

func TestIsTempName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{".foo.md.tmp-abc123", true},
		{".foo.md.tmp-a", true},
		{"foo.md.tmp-abc123", false},  // missing leading dot
		{".foo.md.tmp-", false},       // empty hex suffix
		{".foo.tmp-1.md", false},      // suffix after tmp- isn't pure hex
		{".foo.md.tmp-abcXYZ", false}, // non-hex characters in suffix
		{"home-garden.md", false},
		{".not-a-tmp-file", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTempName(tt.name); got != tt.want {
				t.Errorf("isTempName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestSweepTemp_RespectsOlderThan(t *testing.T) {
	l := Layout{Projects: "/data/ingot/projects"}
	mem := fsx.NewMem()
	if err := mem.MkdirAll(l.Projects, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	f, err := mem.Create(filepath.Join(l.Projects, ".foo.md.tmp-abc"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// olderThan is huge, so the temp file (created moments ago) isn't
	// old enough to sweep yet.
	if err := SweepTemp(mem, l, 24*time.Hour); err != nil {
		t.Fatalf("SweepTemp() error = %v", err)
	}

	entries, err := mem.ReadDir(l.Projects)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("SweepTemp removed a temp file younger than olderThan; entries = %v", entries)
	}
}

func TestSweepTemp_MissingProjectsDirIsNotAnError(t *testing.T) {
	l := Layout{Projects: "/data/ingot/projects"}
	mem := fsx.NewMem()

	if err := SweepTemp(mem, l, 0); err != nil {
		t.Fatalf("SweepTemp() error = %v, want nil for a missing Projects dir", err)
	}
}
