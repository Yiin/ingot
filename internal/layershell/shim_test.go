package layershell

import "testing"

// TestEnumsMatchTheHeader guards against a gtk4-layer-shell version bump
// silently reordering its C enums under us. These values are copied
// straight from gtk4-layer-shell.h (1.3.0, the version this repo builds
// and links against) and must never be "improved" independently of it.
func TestEnumsMatchTheHeader(t *testing.T) {
	layers := map[Layer]int{
		LayerBackground: 0,
		LayerBottom:     1,
		LayerTop:        2,
		LayerOverlay:    3,
	}
	for got, want := range layers {
		if int(got) != want {
			t.Errorf("Layer constant = %d, want %d", got, want)
		}
	}

	edges := map[Edge]int{
		EdgeLeft:   0,
		EdgeRight:  1,
		EdgeTop:    2,
		EdgeBottom: 3,
	}
	for got, want := range edges {
		if int(got) != want {
			t.Errorf("Edge constant = %d, want %d", got, want)
		}
	}

	modes := map[KeyboardMode]int{
		KeyboardModeNone:      0,
		KeyboardModeExclusive: 1,
		KeyboardModeOnDemand:  2,
	}
	for got, want := range modes {
		if int(got) != want {
			t.Errorf("KeyboardMode constant = %d, want %d", got, want)
		}
	}
}
