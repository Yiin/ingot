package config

import (
	"encoding/json"
	"path/filepath"

	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/paths"
)

// PanelState is Ingot's small persisted UI preference set, kept in
// $XDG_STATE_HOME/ingot/panel.json — deliberately its own file, not a
// new field on fsstore's state.json: that file already has a single
// owner (internal/store/fsstore/persist.go, ActiveProject), and giving
// it a second, unrelated writer would mean either package's write can
// race the other's read-modify-write cycle. A UI preference has no
// business in the store's own note-and-project domain state anyway.
//
// Ingot's layer-shell panel has no user-facing resize or move affordance
// today (fixed width, right-edge anchored, height capped by theme
// constants — see internal/layershell), so there is no literal size or
// position to persist yet; KeepOnTop is the one piece of panel-level UI
// state that already exists (menus.Handlers.SetKeepOnTop) and needs
// somewhere to survive a restart.
type PanelState struct {
	KeepOnTop bool `json:"keepOnTop,omitempty"`
}

// LoadPanelState reads panel.json, or returns the zero PanelState if the
// file is missing, unreadable, or malformed — a corrupt or absent
// preference file is never fatal, the same tolerance Load extends to a
// missing config.toml.
func LoadPanelState(fsys fsx.FS, layout paths.Layout) PanelState {
	if layout.State == "" {
		return PanelState{}
	}
	raw, err := fsys.ReadFile(panelStatePath(layout))
	if err != nil {
		return PanelState{}
	}
	var st PanelState
	if err := json.Unmarshal(raw, &st); err != nil {
		return PanelState{}
	}
	return st
}

// SavePanelState atomically writes st to panel.json, creating
// $XDG_STATE_HOME/ingot if it does not already exist.
func SavePanelState(fsys fsx.FS, layout paths.Layout, st PanelState) error {
	if layout.State == "" {
		return nil
	}
	if err := fsys.MkdirAll(layout.State, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return fsx.AtomicWrite(fsys, panelStatePath(layout), data)
}

func panelStatePath(layout paths.Layout) string {
	return filepath.Join(layout.State, "panel.json")
}
