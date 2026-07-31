package motion

import "testing"

// TestDurationsMatchSpecTable pins every constant to the child spec's own
// table (copper-l2z.29) verbatim — a change here must be a deliberate
// re-measurement, never an accidental drift.
func TestDurationsMatchSpecTable(t *testing.T) {
	cases := []struct {
		name string
		got  interface{ Milliseconds() int64 }
		want int64
	}{
		{"RowInsertDuration", RowInsertDuration, 180},
		{"RowRemoveDuration", RowRemoveDuration, 140},
		{"HoverInDuration", HoverInDuration, 90},
		{"HoverOutDuration", HoverOutDuration, 140},
		{"FocusRingDuration", FocusRingDuration, 120},
		{"SelectionFillDuration", SelectionFillDuration, 120},
		{"CheckboxFillDuration", CheckboxFillDuration, 120},
		{"CheckboxTickDuration", CheckboxTickDuration, 140},
		{"CheckboxTickOverlap", CheckboxTickOverlap, 40},
		{"CheckboxTotalDuration", CheckboxTotalDuration, 220},
		{"StrikethroughDuration", StrikethroughDuration, 200},
		{"ComposerGrowthDuration", ComposerGrowthDuration, 120},
		{"ToastInDuration", ToastInDuration, 140},
		{"ToastHoldDuration", ToastHoldDuration, 1200},
		{"ToastOutDuration", ToastOutDuration, 120},
		{"ContextMenuOpenDuration", ContextMenuOpenDuration, 90},
		{"ContextMenuDismissDuration", ContextMenuDismissDuration, 140},
		{"PanelShowDuration", PanelShowDuration, 200},
		{"PanelHideDuration", PanelHideDuration, 150},
		{"ScrollToInsertedDuration", ScrollToInsertedDuration, 240},
		{"ScrollbarInDuration", ScrollbarInDuration, 80},
		{"ScrollbarHoldDuration", ScrollbarHoldDuration, 700},
		{"ScrollbarOutDuration", ScrollbarOutDuration, 300},
	}
	for _, c := range cases {
		if got := c.got.Milliseconds(); got != c.want {
			t.Errorf("%s = %dms, want %dms", c.name, got, c.want)
		}
	}
}
