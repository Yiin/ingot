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

	want := PanelState{KeepOnTop: true}
	if err := SavePanelState(fs, layout, want); err != nil {
		t.Fatalf("SavePanelState: %v", err)
	}

	got := LoadPanelState(fs, layout)
	if got != want {
		t.Errorf("LoadPanelState after save = %+v, want %+v", got, want)
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
