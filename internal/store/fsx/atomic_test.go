package fsx

import (
	"errors"
	"testing"
)

func TestAtomicWrite_Succeeds(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := "/data/projects/foo.md"

	if err := AtomicWrite(m, path, []byte("new content")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	got, err := m.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "new content" {
		t.Errorf("ReadFile = %q, want %q", got, "new content")
	}

	// No leftover temp file.
	entries, err := m.ReadDir("/data/projects")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ReadDir found %d entries after AtomicWrite, want 1 (no leftover temp file)", len(entries))
	}
}

// TestAtomicWrite_CrashAtEverySyscallLeavesLiveFileIntact fails each
// syscall AtomicWrite makes, one at a time, and asserts that in every
// case the live path afterward holds either the exact old bytes (if the
// failure happened before the rename) or the exact new bytes (if it
// happened at or after the rename) — never anything else.
func TestAtomicWrite_CrashAtEverySyscallLeavesLiveFileIntact(t *testing.T) {
	const (
		oldContent = "old content"
		newContent = "brand new content, different length"
	)
	injectedErr := errors.New("simulated crash")

	// tmpWildcard matches AtomicWrite's randomly-suffixed temp file
	// without needing to predict the random suffix.
	const tmpWildcard = "/data/projects/.foo.md.tmp-*"

	steps := []struct {
		op        string
		path      string
		expectOld bool // true: live must still hold oldContent
		expectNew bool // true: live must now hold newContent
	}{
		{op: "Create", path: tmpWildcard, expectOld: true},
		{op: "Write", path: tmpWildcard, expectOld: true},
		{op: "Sync", path: tmpWildcard, expectOld: true},
		{op: "Close", path: tmpWildcard, expectOld: true},
		{op: "Rename", path: tmpWildcard, expectOld: true},
		// SyncDir fails only after the rename already landed, so the new
		// content is already live even though AtomicWrite reports an
		// error.
		{op: "SyncDir", path: "/data/projects", expectNew: true},
	}

	for _, step := range steps {
		t.Run(step.op, func(t *testing.T) {
			m := NewMem()
			if err := m.MkdirAll("/data/projects", 0o755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			path := "/data/projects/foo.md"
			writeFile(t, m, path, oldContent)

			m.FailOn(step.op, step.path, injectedErr)

			err := AtomicWrite(m, path, []byte(newContent))
			if err == nil {
				t.Fatalf("AtomicWrite with %s failure returned nil error", step.op)
			}
			if !errors.Is(err, injectedErr) {
				t.Fatalf("AtomicWrite error = %v, want it to wrap %v", err, injectedErr)
			}

			got, rerr := m.ReadFile(path)
			if rerr != nil {
				t.Fatalf("ReadFile(live) after %s failure: %v", step.op, rerr)
			}
			switch {
			case step.expectOld && string(got) != oldContent:
				t.Errorf("after %s failure, live = %q, want unchanged %q", step.op, got, oldContent)
			case step.expectNew && string(got) != newContent:
				t.Errorf("after %s failure, live = %q, want %q", step.op, got, newContent)
			}
		})
	}
}

func TestAtomicWrite_CleansUpTempFileOnFailure(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := "/data/projects/foo.md"
	writeFile(t, m, path, "old")

	m.FailOn("Sync", "/data/projects/.foo.md.tmp-*", errors.New("simulated crash"))

	if err := AtomicWrite(m, path, []byte("new")); err == nil {
		t.Fatal("AtomicWrite returned nil error, want the injected failure")
	}

	entries, err := m.ReadDir("/data/projects")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ReadDir found %d entries after a failed AtomicWrite, want 1 (temp file cleaned up); entries: %v", len(entries), entries)
	}
}

func TestAtomicWrite_FirstWriteNoPreexistingLiveFile(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := "/data/projects/foo.md"

	if err := AtomicWrite(m, path, []byte("first")); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	got, err := m.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "first" {
		t.Errorf("ReadFile = %q, want %q", got, "first")
	}
}
