package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCapabilities(t *testing.T, sysDir, event, keyMask string) {
	t.Helper()
	dir := filepath.Join(sysDir, event, "device", "capabilities")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "key"), []byte(keyMask+"\n"), 0o444); err != nil {
		t.Fatal(err)
	}
}

func TestProbeKeyboards(t *testing.T) {
	sysDir := t.TempDir()
	devDir := t.TempDir()

	// event0: keyboard capability (bits 42 and 54 set), device node
	// present and openable.
	writeCapabilities(t, sysDir, "event0", "fffffffffffffffe")
	if err := os.WriteFile(filepath.Join(devDir, "event0"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// event1: keyboard capability, but no /dev node — simulates a
	// permission-denied or not-yet-created device.
	writeCapabilities(t, sysDir, "event1", "fffffffffffffffe")

	// event2: real multi-word capability dump (captured shape from a
	// live device), last word carries the shift bits.
	writeCapabilities(t, sysDir, "event2", "1000000000007 ff98007a000007ff febeffdfffefffff fffffffffffffffe")
	if err := os.WriteFile(filepath.Join(devDir, "event2"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// event3: a mouse — no shift bits set, must not count as a keyboard
	// even though its /dev node is openable.
	writeCapabilities(t, sysDir, "event3", "0")
	if err := os.WriteFile(filepath.Join(devDir, "event3"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// event4: sysfs entry with no capabilities file at all — must be
	// skipped, not fail the whole probe.
	if err := os.MkdirAll(filepath.Join(sysDir, "event4", "device"), 0o755); err != nil {
		t.Fatal(err)
	}

	// A non-"event*" sysfs entry must be ignored entirely.
	if err := os.MkdirAll(filepath.Join(sysDir, "mouse0"), 0o755); err != nil {
		t.Fatal(err)
	}

	status, err := ProbeKeyboards(sysDir, devDir)
	if err != nil {
		t.Fatalf("ProbeKeyboards() error = %v", err)
	}
	if status.Detected != 3 {
		t.Errorf("Detected = %d, want 3 (event0, event1, event2)", status.Detected)
	}
	if status.Readable != 2 {
		t.Errorf("Readable = %d, want 2 (event0, event2)", status.Readable)
	}
}

func TestProbeKeyboards_MissingSysDir(t *testing.T) {
	if _, err := ProbeKeyboards(filepath.Join(t.TempDir(), "does-not-exist"), t.TempDir()); err == nil {
		t.Error("ProbeKeyboards() with a missing sysDir: error = nil, want an error")
	}
}

func TestIsKeyboardCapable(t *testing.T) {
	sysDir := t.TempDir()
	writeCapabilities(t, sysDir, "event0", "fffffffffffffffe")
	writeCapabilities(t, sysDir, "event1", "0")

	got, err := isKeyboardCapable(filepath.Join(sysDir, "event0"))
	if err != nil || !got {
		t.Errorf("isKeyboardCapable(event0) = (%v, %v), want (true, nil)", got, err)
	}
	got, err = isKeyboardCapable(filepath.Join(sysDir, "event1"))
	if err != nil || got {
		t.Errorf("isKeyboardCapable(event1) = (%v, %v), want (false, nil)", got, err)
	}
}
