package fsx

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestOSFS_AtomicWriteAndRotateBackup(t *testing.T) {
	dir := t.TempDir()
	fsys := OS()
	projects := filepath.Join(dir, "projects")
	backups := filepath.Join(dir, "backups")
	if err := fsys.MkdirAll(projects, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	live := filepath.Join(projects, "foo.md")
	if err := AtomicWrite(fsys, live, []byte("v1")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	got, err := os.ReadFile(live)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if string(got) != "v1" {
		t.Fatalf("live content = %q, want %q", got, "v1")
	}

	// No leftover temp file on real disk either.
	entries, err := os.ReadDir(projects)
	if err != nil {
		t.Fatalf("os.ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("projects dir has %d entries after AtomicWrite, want 1", len(entries))
	}

	if err := RotateBackup(fsys, live, backups, 3); err != nil {
		t.Fatalf("RotateBackup: %v", err)
	}
	backupGot, err := os.ReadFile(filepath.Join(backups, "foo.md.1"))
	if err != nil {
		t.Fatalf("os.ReadFile(backup): %v", err)
	}
	if string(backupGot) != "v1" {
		t.Fatalf("backup content = %q, want %q", backupGot, "v1")
	}

	if err := AtomicWrite(fsys, live, []byte("v2")); err != nil {
		t.Fatalf("AtomicWrite (second): %v", err)
	}
	got, err = os.ReadFile(live)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if string(got) != "v2" {
		t.Fatalf("live content = %q, want %q", got, "v2")
	}
}

func TestOSFS_ReadFileMissingReturnsNotExist(t *testing.T) {
	fsys := OS()
	if _, err := fsys.ReadFile(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile(missing) error = %v, want fs.ErrNotExist", err)
	}
}

func TestOSFS_SweepTempIntegration(t *testing.T) {
	dir := t.TempDir()
	fsys := OS()
	if err := fsys.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "live.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".live.md.tmp-abc"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	entries, err := fsys.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ReadDir returned %d entries, want 2", len(entries))
	}
}
