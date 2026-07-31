package fsx

import (
	"errors"
	"io/fs"
	"syscall"
	"testing"
)

func writeFile(t *testing.T, m *MemFS, path, content string) {
	t.Helper()
	f, err := m.Create(path)
	if err != nil {
		t.Fatalf("Create(%s): %v", path, err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatalf("Write(%s): %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close(%s): %v", path, err)
	}
}

func TestMemFS_CreateReadFile(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, m, "/data/a.txt", "hello")

	got, err := m.ReadFile("/data/a.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("ReadFile = %q, want %q", got, "hello")
	}
}

func TestMemFS_CreateExistingPathFailsLikeOSFile(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, m, "/data/a.txt", "hello")

	// osFS.Create uses O_EXCL deliberately (see osfs.go); MemFS must
	// fail the same way instead of silently truncating, or a test that
	// passes against MemFS could still crash against the real
	// filesystem.
	if _, err := m.Create("/data/a.txt"); !errors.Is(err, fs.ErrExist) {
		t.Errorf("Create(existing path) error = %v, want fs.ErrExist", err)
	}
	got, err := m.ReadFile("/data/a.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("ReadFile after failed Create = %q, want original content %q unchanged", got, "hello")
	}
}

func TestMemFS_ReadFileMissingReturnsNotExist(t *testing.T) {
	m := NewMem()
	if _, err := m.ReadFile("/nope"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile(missing) error = %v, want fs.ErrNotExist", err)
	}
}

func TestMemFS_Rename(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, m, "/data/tmp", "content")

	if err := m.Rename("/data/tmp", "/data/live"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, err := m.ReadFile("/data/tmp"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile(old path after rename) error = %v, want fs.ErrNotExist", err)
	}
	got, err := m.ReadFile("/data/live")
	if err != nil {
		t.Fatalf("ReadFile(new path): %v", err)
	}
	if string(got) != "content" {
		t.Errorf("ReadFile(new path) = %q, want %q", got, "content")
	}
}

func TestMemFS_Link(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, m, "/data/a", "original")

	if err := m.Link("/data/a", "/data/b"); err != nil {
		t.Fatalf("Link: %v", err)
	}
	got, err := m.ReadFile("/data/b")
	if err != nil {
		t.Fatalf("ReadFile(linked): %v", err)
	}
	if string(got) != "original" {
		t.Errorf("ReadFile(linked) = %q, want %q", got, "original")
	}
	// Both paths must still resolve independently after the source is
	// removed — Link is a copy in the memory model, not an alias.
	if err := m.Remove("/data/a"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := m.ReadFile("/data/b"); err != nil {
		t.Errorf("ReadFile(linked) after removing source: %v", err)
	}
}

func TestMemFS_ReadDir(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, m, "/data/projects/a.md", "a")
	writeFile(t, m, "/data/projects/b.md", "b")

	entries, err := m.ReadDir("/data/projects")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ReadDir returned %d entries, want 2", len(entries))
	}
	if entries[0].Name() != "a.md" || entries[1].Name() != "b.md" {
		t.Errorf("ReadDir entries = [%s, %s], want sorted [a.md, b.md]", entries[0].Name(), entries[1].Name())
	}
}

func TestMemFS_RemoveNonEmptyDirFails(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, m, "/data/projects/a.md", "a")

	if err := m.Remove("/data/projects"); !errors.Is(err, syscall.ENOTEMPTY) {
		t.Errorf("Remove(non-empty dir) error = %v, want syscall.ENOTEMPTY", err)
	}
	if _, err := m.ReadFile("/data/projects/a.md"); err != nil {
		t.Errorf("child file gone after a failed Remove: %v", err)
	}
}

func TestMemFS_RemoveEmptyDirSucceeds(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := m.Remove("/data/projects"); err != nil {
		t.Fatalf("Remove(empty dir): %v", err)
	}
	if _, err := m.Stat("/data/projects"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat after Remove error = %v, want fs.ErrNotExist", err)
	}
}

func TestMemFS_StatDirVsFile(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, m, "/data/projects/a.md", "hello")

	dirInfo, err := m.Stat("/data/projects")
	if err != nil {
		t.Fatalf("Stat(dir): %v", err)
	}
	if !dirInfo.IsDir() {
		t.Error("Stat(dir).IsDir() = false, want true")
	}

	fileInfo, err := m.Stat("/data/projects/a.md")
	if err != nil {
		t.Fatalf("Stat(file): %v", err)
	}
	if fileInfo.IsDir() {
		t.Error("Stat(file).IsDir() = true, want false")
	}
	if fileInfo.Size() != 5 {
		t.Errorf("Stat(file).Size() = %d, want 5", fileInfo.Size())
	}
}

func TestMemFS_FailOnExactPath(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	wantErr := errors.New("injected failure")
	m.FailOn("Create", "/data/a", wantErr)

	if _, err := m.Create("/data/a"); !errors.Is(err, wantErr) {
		t.Errorf("Create error = %v, want %v", err, wantErr)
	}
	// A different path must be unaffected.
	if _, err := m.Create("/data/b"); err != nil {
		t.Errorf("Create(/data/b) error = %v, want nil", err)
	}
}

func TestMemFS_FailOnPrefixWildcard(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	wantErr := errors.New("injected failure")
	m.FailOn("Create", "/data/.a.tmp-*", wantErr)

	if _, err := m.Create("/data/.a.tmp-abc123"); !errors.Is(err, wantErr) {
		t.Errorf("Create(matching wildcard) error = %v, want %v", err, wantErr)
	}
	if _, err := m.Create("/data/a"); err != nil {
		t.Errorf("Create(non-matching) error = %v, want nil", err)
	}
}

func TestMemFS_Observer(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	var ops []string
	m.SetObserver(func(op, path string) {
		ops = append(ops, op+":"+path)
	})

	writeFile(t, m, "/data/a", "x")

	want := []string{"Create:/data/a", "Write:/data/a", "Close:/data/a"}
	if len(ops) != len(want) {
		t.Fatalf("observed ops = %v, want %v", ops, want)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Errorf("ops[%d] = %q, want %q", i, ops[i], want[i])
		}
	}
}
