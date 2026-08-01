package app

import (
	"testing"

	"github.com/Yiin/ingot/internal/config"
	"github.com/Yiin/ingot/internal/ui/theme"
)

// TestPanelSize covers the fallback panelSize applies per axis. A zero is
// what LoadPanelState returns both for a missing panel.json and for one
// written before the panel became a resizable toplevel, so "0 means use
// the design's own default" is the contract that keeps a first run and an
// upgrade opening at the same size.
//
// The two mixed cases are not hypothetical padding: the axes fall back
// independently, so a file carrying only one of them must not drag the
// other to zero and collapse the window.
func TestPanelSize(t *testing.T) {
	tests := []struct {
		name                  string
		state                 config.PanelState
		wantWidth, wantHeight int
	}{
		{
			name:       "unset falls back to the design defaults",
			state:      config.PanelState{},
			wantWidth:  theme.PanelWidth,
			wantHeight: theme.PanelHeight,
		},
		{
			name:       "a saved size wins over both defaults",
			state:      config.PanelState{Width: 480, Height: 900},
			wantWidth:  480,
			wantHeight: 900,
		},
		{
			name:       "only width saved leaves height on its default",
			state:      config.PanelState{Width: 480},
			wantWidth:  480,
			wantHeight: theme.PanelHeight,
		},
		{
			name:       "only height saved leaves width on its default",
			state:      config.PanelState{Height: 900},
			wantWidth:  theme.PanelWidth,
			wantHeight: 900,
		},
		{
			name:       "a negative size is treated as unset, not honoured",
			state:      config.PanelState{Width: -1, Height: -1},
			wantWidth:  theme.PanelWidth,
			wantHeight: theme.PanelHeight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &App{panelState: tt.state}
			w, h := a.panelSize()
			if w != tt.wantWidth || h != tt.wantHeight {
				t.Errorf("panelSize() = %dx%d, want %dx%d", w, h, tt.wantWidth, tt.wantHeight)
			}
		})
	}
}

// TestSavePanelSizeWithoutAWindowIsANoOp guards the shutdown path.
// shutdown calls savePanelSize before anything else, and it runs even
// when startup failed partway and never built a window — reading a nil
// one would panic during teardown, turning a startup error into a crash.
func TestSavePanelSizeWithoutAWindowIsANoOp(t *testing.T) {
	a := &App{panelState: config.PanelState{Width: 480, Height: 900}}
	a.savePanelSize()
	if a.panelState.Width != 480 || a.panelState.Height != 900 {
		t.Errorf("savePanelSize with no window changed the state to %dx%d",
			a.panelState.Width, a.panelState.Height)
	}
}
