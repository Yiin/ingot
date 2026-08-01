package config

import (
	"testing"

	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/paths"
)

func TestPanelState_MissingFileReturnsZeroValue(t *testing.T) {
	fs := fsx.NewMem()
	layout := paths.Layout{State: "/state"}

	got := LoadPanelState(fs, layout)
	if got != (PanelState{}) {
		t.Errorf("LoadPanelState = %+v, want zero value", got)
	}
}

func TestPanelState_RoundTrips(t *testing.T) {
	fs := fsx.NewMem()
	layout := paths.Layout{State: "/state"}

	want := PanelState{KeepOnTop: true, Width: 480, Height: 720}
	if err := SavePanelState(fs, layout, want); err != nil {
		t.Fatalf("SavePanelState: %v", err)
	}

	got := LoadPanelState(fs, layout)
	if got != want {
		t.Errorf("LoadPanelState after save = %+v, want %+v", got, want)
	}
}

// TestPanelState_PreSizeFileStillLoads pins the upgrade path: a panel.json
// written before the panel became a resizable toplevel has no width or
// height key at all, and must load as "never set" rather than failing —
// internal/app reads a zero here as "use the design's own default size".
func TestPanelState_PreSizeFileStillLoads(t *testing.T) {
	fs := fsx.NewMem()
	layout := paths.Layout{State: "/state"}
	writeFile(t, fs, "/state/panel.json", `{"keepOnTop": true}`)

	got := LoadPanelState(fs, layout)
	if want := (PanelState{KeepOnTop: true}); got != want {
		t.Errorf("LoadPanelState = %+v, want %+v", got, want)
	}
}

func TestPanelState_CorruptFileReturnsZeroValue(t *testing.T) {
	fs := fsx.NewMem()
	layout := paths.Layout{State: "/state"}
	writeFile(t, fs, "/state/panel.json", "not json")

	got := LoadPanelState(fs, layout)
	if got != (PanelState{}) {
		t.Errorf("LoadPanelState = %+v, want zero value", got)
	}
}
