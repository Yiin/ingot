package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/paths"
)

// Load reads config.toml from layout.Config through fsys, applying every
// recognized key on top of Default(). A missing file is not an error —
// Load returns Default() unchanged, since config.toml is optional and
// never written by Ingot itself (the whole point of keeping it separate
// from state: nothing here ever clobbers a human's hand edits or
// comments). Anything Load could not apply — a malformed line, an
// unknown key, a value that doesn't fit its field — is reported as a
// warning rather than a failure; Load always returns a usable Config.
func Load(fsys fsx.FS, layout paths.Layout) (Config, []string, error) {
	cfg := Default()
	if layout.Config == "" {
		return cfg, nil, nil
	}

	path := filepath.Join(layout.Config, "config.toml")
	raw, err := fsys.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil, nil
		}
		return cfg, nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	var warnings []string
	section := ""
	for i, line := range strings.Split(string(raw), "\n") {
		lineNo := i + 1
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if name, ok := sectionHeader(line); ok {
			section = name
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			warnings = append(warnings, fmt.Sprintf("line %d: not a key = value line, ignored", lineNo))
			continue
		}
		key = strings.TrimSpace(key)
		value = unquote(strings.TrimSpace(value))

		if section == "keys" {
			cfg.Keys[key] = value
			continue
		}

		switch key {
		case "hotkey_window":
			d, err := time.ParseDuration(value)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("line %d: hotkey_window %q is not a valid duration, ignored", lineNo, value))
				continue
			}
			cfg.Hotkey.Window = d
		case "panel_toggle_binding":
			cfg.PanelToggleBinding = value
		case "theme":
			cfg.Theme = value
		default:
			warnings = append(warnings, fmt.Sprintf("line %d: unknown key %q, ignored", lineNo, key))
		}
	}

	return cfg, warnings, nil
}

// sectionHeader reports whether line is a TOML-style "[name]" section
// header, and if so, name. Only "[keys]" means anything to Load today;
// a key = value line under any other section falls through to the
// top-level switch below and is reported as an unknown key, the same as
// it would be with no section at all.
func sectionHeader(line string) (name string, ok bool) {
	if len(line) < 2 || line[0] != '[' || line[len(line)-1] != ']' {
		return "", false
	}
	return strings.TrimSpace(line[1 : len(line)-1]), true
}

// unquote strips one layer of matching double quotes, so both
// `theme = "light"` and `theme = light` are accepted.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
