package fsx

import (
	"syscall"
	"testing"
)

func TestRotateBackup_FirstWrite(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	live := "/data/projects/foo.md"
	writeFile(t, m, live, "v1")

	if err := RotateBackup(m, live, "/data/backups", 3); err != nil {
		t.Fatalf("RotateBackup: %v", err)
	}

	got, err := m.ReadFile("/data/backups/foo.md.1")
	if err != nil {
		t.Fatalf("ReadFile(.1): %v", err)
	}
	if string(got) != "v1" {
		t.Errorf("backups/foo.md.1 = %q, want %q", got, "v1")
	}
	if _, err := m.Stat("/data/backups/foo.md.2"); err == nil {
		t.Error("backups/foo.md.2 exists after a first write, want it absent")
	}
}

func TestRotateBackup_FullRotationDropsOldest(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := m.MkdirAll("/data/backups", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	live := "/data/projects/foo.md"
	writeFile(t, m, live, "v4")
	writeFile(t, m, "/data/backups/foo.md.1", "v3")
	writeFile(t, m, "/data/backups/foo.md.2", "v2")
	writeFile(t, m, "/data/backups/foo.md.3", "v1")

	if err := RotateBackup(m, live, "/data/backups", 3); err != nil {
		t.Fatalf("RotateBackup: %v", err)
	}

	for slot, want := range map[string]string{
		"/data/backups/foo.md.1": "v4",
		"/data/backups/foo.md.2": "v3",
		"/data/backups/foo.md.3": "v2",
	} {
		got, err := m.ReadFile(slot)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", slot, err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q (old .3 = %q should have been dropped)", slot, got, want, "v1")
		}
	}
}

func TestRotateBackup_FallsBackToCopyOnEXDEV(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	live := "/data/projects/foo.md"
	writeFile(t, m, live, "cross-device content")

	m.FailOn("Link", live, syscall.EXDEV)

	if err := RotateBackup(m, live, "/data/backups", 3); err != nil {
		t.Fatalf("RotateBackup: %v", err)
	}

	got, err := m.ReadFile("/data/backups/foo.md.1")
	if err != nil {
		t.Fatalf("ReadFile(.1): %v", err)
	}
	if string(got) != "cross-device content" {
		t.Errorf("backups/foo.md.1 = %q, want %q", got, "cross-device content")
	}
}

func TestRotateBackup_FallsBackToCopyOnEPERM(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	live := "/data/projects/foo.md"
	writeFile(t, m, live, "no-permission content")

	m.FailOn("Link", live, syscall.EPERM)

	if err := RotateBackup(m, live, "/data/backups", 3); err != nil {
		t.Fatalf("RotateBackup: %v", err)
	}

	got, err := m.ReadFile("/data/backups/foo.md.1")
	if err != nil {
		t.Fatalf("ReadFile(.1): %v", err)
	}
	if string(got) != "no-permission content" {
		t.Errorf("backups/foo.md.1 = %q, want %q", got, "no-permission content")
	}
}

func TestRotateBackup_PropagatesOtherLinkErrors(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	live := "/data/projects/foo.md"
	writeFile(t, m, live, "v1")

	m.FailOn("Link", live, syscall.ENOSPC)

	if err := RotateBackup(m, live, "/data/backups", 3); err == nil {
		t.Fatal("RotateBackup with a non-EXDEV/EPERM Link failure returned nil error, want it propagated")
	}
}

func TestRotateBackup_KeepZeroIsNoop(t *testing.T) {
	m := NewMem()
	if err := m.MkdirAll("/data/projects", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	live := "/data/projects/foo.md"
	writeFile(t, m, live, "v1")

	if err := RotateBackup(m, live, "/data/backups", 0); err != nil {
		t.Fatalf("RotateBackup: %v", err)
	}
	if _, err := m.Stat("/data/backups"); err == nil {
		t.Error("RotateBackup(keep=0) created /data/backups, want no-op")
	}
}
