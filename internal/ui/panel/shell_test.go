package panel

import (
	"testing"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// TestToastBottomInset locks in the arithmetic a live-display test can't
// easily assert on: the composer's own top/bottom CSS padding
// (2*theme.CardPadY) and the panel's own bottom padding
// (theme.PanelPadBottom) both sit between composer.OnHeightChanged's
// reported content height and the panel's true outer bottom edge, on
// top of the PanelToastGap clearance above the composer's own top edge.
func TestToastBottomInset(t *testing.T) {
	want := theme.ComposerMinHeight + 2*theme.CardPadY + theme.PanelPadBottom + theme.PanelToastGap
	if got := toastBottomInset(theme.ComposerMinHeight); got != want {
		t.Errorf("toastBottomInset(%d) = %d, want %d", theme.ComposerMinHeight, got, want)
	}

	grown := theme.ComposerMinHeight + theme.LineBody
	if got, want := toastBottomInset(grown), toastBottomInset(theme.ComposerMinHeight)+theme.LineBody; got != want {
		t.Errorf("toastBottomInset(%d) = %d, want %d (grows 1:1 with composer height)", grown, got, want)
	}
}
