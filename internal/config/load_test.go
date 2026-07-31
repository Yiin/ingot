package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/paths"
)

func TestLoad_MissingFileReturnsDefault(t *testing.T) {
	fs := fsx.NewMem()
	layout := paths.Layout{Config: "/config"}

	cfg, warnings, err := Load(fs, layout)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if warnings != nil {
		t.Errorf("warnings = %v, want nil", warnings)
	}
	if cfg != Default() {
		t.Errorf("cfg = %+v, want Default() = %+v", cfg, Default())
	}
}

func TestLoad_OverridesRecognizedKeys(t *testing.T) {
	fs := fsx.NewMem()
	layout := paths.Layout{Config: "/config"}
	writeFile(t, fs, "/config/config.toml", `# a comment
hotkey_window = "500ms"
panel_toggle_binding = "<Super>space"
theme = "light"
`)

	cfg, warnings, err := Load(fs, layout)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if cfg.Hotkey.Window != 500*time.Millisecond {
		t.Errorf("Hotkey.Window = %v, want 500ms", cfg.Hotkey.Window)
	}
	if cfg.PanelToggleBinding != "<Super>space" {
		t.Errorf("PanelToggleBinding = %q, want %q", cfg.PanelToggleBinding, "<Super>space")
	}
}

func TestLoad_WarnsOnBadLines(t *testing.T) {
	fs := fsx.NewMem()
	layout := paths.Layout{Config: "/config"}
	writeFile(t, fs, "/config/config.toml", `not a valid line
hotkey_window = "not-a-duration"
made_up_key = 1
`)

	cfg, warnings, err := Load(fs, layout)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(warnings) != 3 {
		t.Fatalf("warnings = %v, want 3", warnings)
	}
	if cfg.Hotkey.Window != DefaultHotkeyWindow {
		t.Errorf("Hotkey.Window = %v, want default %v after a bad value", cfg.Hotkey.Window, DefaultHotkeyWindow)
	}
}

func writeFile(t *testing.T, fsys fsx.FS, path, content string) {
	t.Helper()
	if err := fsys.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	f, err := fsys.Create(path)
	if err != nil {
		t.Fatalf("Create %s: %v", path, err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatalf("Write %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close %s: %v", path, err)
	}
}
