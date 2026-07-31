package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// Layout is every directory Ingot reads or writes.
type Layout struct {
	Data     string // $XDG_DATA_HOME/ingot
	Projects string // Data/projects — one Markdown file per project
	Meta     string // Data/meta — advisory provenance sidecars
	Backups  string // Data/backups — rotated .1/.2/.3 copies
	Trash    string // Data/trash — deleted projects, one undo level
	Config   string // $XDG_CONFIG_HOME/ingot — hand-edited config.toml
	State    string // $XDG_STATE_HOME/ingot — state.json, ingot.log
}

// Resolve computes Layout from the environment. For each of data, config,
// and state, it honors an INGOT_*_DIR override (the directory Ingot uses
// directly), then the matching XDG_*_HOME base (with "ingot" appended),
// then the XDG default. Per the XDG Base Directory spec, an empty or
// relative value for any of these variables counts as unset; the same
// rule is applied to the INGOT_*_DIR overrides for consistency, since a
// relative override is exactly the kind of thing that silently writes
// files in the wrong place depending on the caller's working directory.
func Resolve() (Layout, error) {
	data, err := resolveDir("INGOT_DATA_DIR", "XDG_DATA_HOME", ".local/share")
	if err != nil {
		return Layout{}, err
	}
	config, err := resolveDir("INGOT_CONFIG_DIR", "XDG_CONFIG_HOME", ".config")
	if err != nil {
		return Layout{}, err
	}
	state, err := resolveDir("INGOT_STATE_DIR", "XDG_STATE_HOME", ".local/state")
	if err != nil {
		return Layout{}, err
	}

	return Layout{
		Data:     data,
		Projects: filepath.Join(data, "projects"),
		Meta:     filepath.Join(data, "meta"),
		Backups:  filepath.Join(data, "backups"),
		Trash:    filepath.Join(data, "trash"),
		Config:   config,
		State:    state,
	}, nil
}

// resolveDir resolves one of the three Ingot directories: overrideEnv
// wins outright if set, otherwise xdgEnv's value has "ingot" appended,
// otherwise $HOME/xdgDefaultRel/ingot.
func resolveDir(overrideEnv, xdgEnv, xdgDefaultRel string) (string, error) {
	if v, ok := absEnv(overrideEnv); ok {
		return v, nil
	}
	if v, ok := absEnv(xdgEnv); ok {
		return filepath.Join(v, "ingot"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("paths: resolve %s: no override, no %s, and home directory is unavailable: %w", overrideEnv, xdgEnv, err)
	}
	return filepath.Join(home, xdgDefaultRel, "ingot"), nil
}

// absEnv reads name and reports whether it's set to a non-empty absolute
// path — the XDG spec's definition of a "set" base directory variable.
func absEnv(name string) (string, bool) {
	v := os.Getenv(name)
	if v == "" || !filepath.IsAbs(v) {
		return "", false
	}
	return v, true
}
