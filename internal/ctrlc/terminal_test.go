package ctrlc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsTerminalClass(t *testing.T) {
	cases := []struct {
		class string
		want  bool
	}{
		{"kitty", true},
		{"Kitty", true},
		{"  alacritty  ", true},
		{"org.wezfurlong.wezterm", true},
		{"com.mitchellh.ghostty", true},
		{"firefox", false},
		{"", false},
		{"code", false},
		{"chromium-browser", false},
	}
	for _, tc := range cases {
		if got := isTerminalClass(tc.class); got != tc.want {
			t.Errorf("isTerminalClass(%q) = %v, want %v", tc.class, got, tc.want)
		}
	}
}

// writeFakeBinary drops an executable shell script named name onto PATH
// (via a temp dir prepended for the duration of the test) that prints
// stdout and exits with code, so hyprctlFocusedClass and runWtypeCtrlC
// can be exercised without a real compositor or wtype install.
func writeFakeBinary(t *testing.T, name, stdout string, code int) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("fake binary script targets linux/bash")
	}
	dir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s' %q\nexit %d\n", stdout, code)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

func TestHyprctlFocusedClassParsesJSON(t *testing.T) {
	writeFakeBinary(t, "hyprctl", `{"class":"kitty","title":"zsh"}`, 0)

	class, err := hyprctlFocusedClass(context.Background())
	if err != nil {
		t.Fatalf("hyprctlFocusedClass: %v", err)
	}
	if class != "kitty" {
		t.Errorf("class = %q, want %q", class, "kitty")
	}
}

func TestHyprctlFocusedClassMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := hyprctlFocusedClass(context.Background()); err == nil {
		t.Fatalf("hyprctlFocusedClass with no hyprctl on PATH: got nil error")
	}
}

func TestHyprctlFocusedClassMalformedJSON(t *testing.T) {
	writeFakeBinary(t, "hyprctl", "not json", 0)
	if _, err := hyprctlFocusedClass(context.Background()); err == nil {
		t.Fatalf("hyprctlFocusedClass with malformed JSON: got nil error")
	}
}
