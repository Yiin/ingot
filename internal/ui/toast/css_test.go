package toast

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// TestFadeInDurationMatchesCSS keeps FadeInDuration in sync with
// style.css's own .toast-in animation length, the same convention
// internal/ui/notelist/css_test.go uses for InsertAnimDuration: the CSS
// class here (hud.go/inpanel.go's enter) is timed to be removed exactly
// when the animation finishes.
func TestFadeInDurationMatchesCSS(t *testing.T) {
	re := regexp.MustCompile(`\.toast-dark\.toast-in,\s*\n\.toast-light\.toast-in\s*\{\s*animation:\s*ingot-toast-in\s+(\d+)ms`)
	m := re.FindStringSubmatch(theme.CSS)
	if m == nil {
		t.Fatalf("style.css has no `.toast-dark.toast-in, .toast-light.toast-in { animation: ingot-toast-in <N>ms ... }` rule")
	}
	assertDurationMatches(t, m[1], FadeInDuration, "FadeInDuration")
}

// TestFadeOutDurationMatchesCSS is TestFadeInDurationMatchesCSS's
// counterpart for the .toast-out exit animation.
func TestFadeOutDurationMatchesCSS(t *testing.T) {
	re := regexp.MustCompile(`\.toast-dark\.toast-out,\s*\n\.toast-light\.toast-out\s*\{\s*animation:\s*ingot-toast-out\s+(\d+)ms`)
	m := re.FindStringSubmatch(theme.CSS)
	if m == nil {
		t.Fatalf("style.css has no `.toast-dark.toast-out, .toast-light.toast-out { animation: ingot-toast-out <N>ms ... }` rule")
	}
	assertDurationMatches(t, m[1], FadeOutDuration, "FadeOutDuration")
}

func assertDurationMatches(t *testing.T, rawMS string, want time.Duration, name string) {
	t.Helper()
	ms, err := strconv.Atoi(rawMS)
	if err != nil {
		t.Fatalf("parsing animation duration %q: %v", rawMS, err)
	}
	if got := time.Duration(ms) * time.Millisecond; got != want {
		t.Errorf("style.css's animation duration = %v, want %v (%s)", got, want, name)
	}
}

// TestToastClassesHaveNoUnconditionalAnimation guards the same bug
// notelist's css_test.go guards for .note-card: an animation declared
// directly on .toast-dark/.toast-light (rather than gated by .toast-in/
// .toast-out) would replay on every CSS recalculation, not just an
// actual show/hide.
func TestToastClassesHaveNoUnconditionalAnimation(t *testing.T) {
	for _, class := range []string{"toast-dark", "toast-light"} {
		re := regexp.MustCompile(`(?s)\.` + class + `\s*\{[^}]*\}`)
		block := re.FindString(theme.CSS)
		if block == "" {
			t.Fatalf("style.css has no bare `.%s { ... }` rule", class)
		}
		if regexp.MustCompile(`animation\s*:`).MatchString(block) {
			t.Errorf(".%s { ... } declares an unconditional animation: %s", class, block)
		}
	}
}
