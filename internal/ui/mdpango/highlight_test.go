package mdpango

import (
	"strings"
	"testing"

	"github.com/diamondburned/gotk4/pkg/pango"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// hl wraps s in the highlight span these tests expect. It reads
// HighlightBackground rather than hardcoding the light accent, so these
// assertions stay about the markup structure and do not quietly pin the
// light colour scheme.
func hl(s string) string {
	return `<span background="` + HighlightBackground() + `">` + s + `</span>`
}

// TestHighlightBackgroundFollowsTheScheme is what actually pins the fix
// for copper-cvj's last hardcoded colour. HighlightBackground used to be
// a package constant, and every other test here would still pass if it
// were reverted to one — they compare rendered markup against itself.
// This one fails the moment the emitted span stops tracking the palette,
// which is the bug: a search match tinted with the light accent on a dark
// card, where a 12% tint is close to invisible.
func TestHighlightBackgroundFollowsTheScheme(t *testing.T) {
	body := "find the needle here"
	start := strings.Index(body, "needle")
	ranges := [][2]int{{start, start + len("needle")}}

	restore := theme.OverrideScheme(theme.SchemeLight)
	light := FullHighlighted(body, ranges)
	restore()

	restore = theme.OverrideScheme(theme.SchemeDark)
	dark := FullHighlighted(body, ranges)
	restore()

	if light == dark {
		t.Fatalf("FullHighlighted rendered identically in both schemes (%q) — the highlight colour is not following the palette", light)
	}
	if want := `background="` + theme.HighlightBg + `"`; !strings.Contains(light, want) {
		t.Errorf("light render = %q, want it to contain %q", light, want)
	}
	if want := `background="` + theme.DarkHighlightBg + `"`; !strings.Contains(dark, want) {
		t.Errorf("dark render = %q, want it to contain %q", dark, want)
	}
}

func TestHighlightComposesWithBold(t *testing.T) {
	body := "This is **bold text** here"
	start := strings.Index(body, "bold")
	end := start + len("bold")

	got := FullHighlighted(body, [][2]int{{start, end}})
	want := `This is <b>` + hl("bold") + ` text</b> here`
	if got != want {
		t.Errorf("FullHighlighted(%q) = %q, want %q", body, got, want)
	}
}

func TestHighlightPlainText(t *testing.T) {
	body := "find the needle in the haystack"
	start := strings.Index(body, "needle")
	end := start + len("needle")

	got := FullHighlighted(body, [][2]int{{start, end}})
	want := `find the ` + hl("needle") + ` in the haystack`
	if got != want {
		t.Errorf("FullHighlighted(%q) = %q, want %q", body, got, want)
	}
}

func TestHighlightSpanningTwoTextRuns(t *testing.T) {
	// "bo*ld* one" is raw bytes b-o-*-l-d-*-space-o-n-e: "bo" is [0,2),
	// the emphasis delimiters are [2,3) and [5,6), "ld" is [3,5). Two
	// ranges either side of the delimiter must produce two separate
	// highlight spans, one plain and one nested inside <i> — writeHighlighted
	// operates per text-node segment, so it can never bridge a tag boundary
	// even if the caller's ranges did.
	body := "bo*ld* one"
	got := FullHighlighted(body, [][2]int{{0, 2}, {3, 5}})
	want := hl("bo") + `<i>` + hl("ld") + `</i> one`
	if got != want {
		t.Errorf("FullHighlighted(%q) = %q, want %q", body, got, want)
	}
}

func TestHighlightNilRangesMatchesUnhighlighted(t *testing.T) {
	bodies := []string{"**bold**", "plain text", "*italic* and `code`"}
	for _, body := range bodies {
		if got, want := FullHighlighted(body, nil), Full(body); got != want {
			t.Errorf("FullHighlighted(%q, nil) = %q, want %q", body, got, want)
		}
		if got, want := CollapsedHighlighted(body, nil), Collapsed(body); got != want {
			t.Errorf("CollapsedHighlighted(%q, nil) = %q, want %q", body, got, want)
		}
	}
}

func TestMergeRangesOverlapping(t *testing.T) {
	body := "catcats"
	// "cat" at [0,3) and "cats" at [3,7) — wait, both match at 0: "cat" at
	// [0,3), "cats" at [3,7) is actually "cats" the literal substring at
	// index 3 is "cats" (c-a-t-s starting at 3: "catcats"[3:7] == "cats").
	// Use overlapping ranges directly instead of relying on substring
	// arithmetic, to keep the test's intent explicit.
	_ = body
	got := mergeRanges([][2]int{{0, 3}, {2, 5}, {5, 5}, {7, 9}})
	want := [][2]int{{0, 5}, {7, 9}}
	if len(got) != len(want) {
		t.Fatalf("mergeRanges = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mergeRanges = %v, want %v", got, want)
		}
	}
}

func TestSafeHighlightedSurvivesHostileInput(t *testing.T) {
	for _, body := range hostileBodies {
		ranges := [][2]int{{0, minInt(3, len(body))}}
		markup := SafeHighlighted(body, ranges)
		stripped := anchorTag.ReplaceAllString(markup, "")
		if _, _, _, err := pango.ParseMarkup(stripped, 0); err != nil {
			t.Fatalf("SafeHighlighted(%q, %v) = %q, pango.ParseMarkup: %v", body, ranges, markup, err)
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
