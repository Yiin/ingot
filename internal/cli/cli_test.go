package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yiin/ingot/internal/store/paths"
)

func TestPathForKind(t *testing.T) {
	layout := paths.Layout{
		Data: "/data", Projects: "/data/projects", Meta: "/data/meta",
		Backups: "/data/backups", Trash: "/data/trash", Config: "/config", State: "/state",
	}
	for _, k := range pathKinds {
		if _, ok := pathForKind(layout, k); !ok {
			t.Errorf("pathForKind(%q) not found, want a Layout field", k)
		}
	}
	if _, ok := pathForKind(layout, "my project"); ok {
		t.Errorf("pathForKind(%q) matched a kind, want false (should fall through to project resolution)", "my project")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "fallback"); got != "fallback" {
		t.Errorf("firstNonEmpty(\"\", ...) = %q, want fallback", got)
	}
	if got := firstNonEmpty("title", "fallback"); got != "title" {
		t.Errorf("firstNonEmpty(title, ...) = %q, want title", got)
	}
}

// withStdout redirects os.Stdout for the duration of fn and returns
// everything written to it — the CLI commands write straight to
// os.Stdout/fmt.Println rather than an injectable writer, matching every
// other command in this file.
func withStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = orig
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return buf.String()
}

func TestRunPath_NoArgsListsEveryKind(t *testing.T) {
	t.Setenv("INGOT_DATA_DIR", t.TempDir())
	t.Setenv("INGOT_CONFIG_DIR", t.TempDir())
	t.Setenv("INGOT_STATE_DIR", t.TempDir())

	out := withStdout(t, func() {
		if err := runPath(nil); err != nil {
			t.Fatalf("runPath(nil): %v", err)
		}
	})
	for _, k := range pathKinds {
		if !strings.Contains(out, k+"\t") {
			t.Errorf("output missing kind %q:\n%s", k, out)
		}
	}
}

func TestRunPath_UnknownProjectErrors(t *testing.T) {
	t.Setenv("INGOT_DATA_DIR", t.TempDir())
	t.Setenv("INGOT_CONFIG_DIR", t.TempDir())
	t.Setenv("INGOT_STATE_DIR", t.TempDir())

	if err := runPath([]string{"no-such-project"}); err == nil {
		t.Fatalf("runPath([no-such-project]) = nil error, want one")
	}
}

func TestImportThenExport_RoundTrips(t *testing.T) {
	data := t.TempDir()
	t.Setenv("INGOT_DATA_DIR", data)
	t.Setenv("INGOT_CONFIG_DIR", t.TempDir())
	t.Setenv("INGOT_STATE_DIR", t.TempDir())

	src := filepath.Join(t.TempDir(), "in.md")
	const body = "---\nid: 0123456789abcdef\ntitle: Groceries\n---\n\n- [ ] milk\n- [x] eggs\n"
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if err := runImport([]string{src}); err != nil {
		t.Fatalf("runImport: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(data, "projects"))
	if err != nil {
		t.Fatalf("read projects dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("projects dir has %d entries, want 1", len(entries))
	}
	slug := strings.TrimSuffix(entries[0].Name(), ".md")

	out := withStdout(t, func() {
		if err := runExport([]string{slug}); err != nil {
			t.Fatalf("runExport: %v", err)
		}
	})
	if !strings.Contains(out, "Groceries") || !strings.Contains(out, "milk") || !strings.Contains(out, "eggs") {
		t.Errorf("exported output missing expected content:\n%s", out)
	}

	// Re-importing the same id without --force must refuse, not silently
	// mint a duplicate project.
	if err := runImport([]string{src}); err == nil {
		t.Errorf("runImport of a duplicate id without --force = nil error, want a refusal")
	}
}
