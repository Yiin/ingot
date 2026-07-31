package notelist

import (
	"math"
	"testing"

	"github.com/Yiin/ingot/internal/ui/theme"
)

func TestSectionTitleIsUppercasedInGo(t *testing.T) {
	cases := map[string]string{
		"To do":     "TO DO",
		"done":      "DONE",
		"Mixed Ca5": "MIXED CA5",
	}
	for in, want := range cases {
		if got := displayTitle(in); got != want {
			t.Errorf("displayTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCapHeightPxIsAFractionOfFontSize(t *testing.T) {
	const fontPx = float64(theme.FontSection)
	got := capHeightPx(fontPx)
	want := fontPx * interCapHeight
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("capHeightPx(%v) = %v, want %v", fontPx, got, want)
	}
	if got <= 0 || got >= fontPx {
		t.Errorf("capHeightPx(%v) = %v, want a value strictly between 0 and the font size", fontPx, got)
	}
}

func TestRuleSitsAboveTheBaselineOnTheCapHeightMidline(t *testing.T) {
	const baseline = 14.0
	got := capMidY(baseline)
	want := baseline - capHeightPx(float64(theme.FontSection))/2
	if got != want {
		t.Errorf("capMidY(%v) = %v, want %v", baseline, got, want)
	}
	if got >= baseline {
		t.Errorf("capMidY(%v) = %v, want a y strictly above the baseline (smaller, since y grows downward)", baseline, got)
	}
}
