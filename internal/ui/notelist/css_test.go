package notelist

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// TestInsertAnimDurationMatchesCSS keeps InsertAnimDuration (the model's
// idea of how long the just-inserted class survives a rebind) in sync
// with style.css's own animation-length, so the strip timer in bindRow
// never fires before or long after the CSS animation actually finishes.
func TestInsertAnimDurationMatchesCSS(t *testing.T) {
	re := regexp.MustCompile(`\.note-card\.just-inserted\s*\{\s*animation:\s*ingot-row-in\s+(\d+)ms`)
	m := re.FindStringSubmatch(theme.CSS)
	if m == nil {
		t.Fatalf("style.css has no `.note-card.just-inserted { animation: ingot-row-in <N>ms ... }` rule")
	}
	ms, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("parsing animation duration %q: %v", m[1], err)
	}
	if got := time.Duration(ms) * time.Millisecond; got != InsertAnimDuration {
		t.Errorf("style.css's ingot-row-in duration = %v, want %v (InsertAnimDuration)", got, InsertAnimDuration)
	}
}

// TestNoteCardHasNoUnconditionalAnimation guards the exact bug the child
// spec warns about: an animation declared directly on .note-card (rather
// than gated by .just-inserted) would replay on every recycled bind, so
// every scroll would re-play the entrance of every visible row.
func TestNoteCardHasNoUnconditionalAnimation(t *testing.T) {
	re := regexp.MustCompile(`(?s)\.note-card\s*\{[^}]*\}`)
	block := re.FindString(theme.CSS)
	if block == "" {
		t.Fatalf("style.css has no bare `.note-card { ... }` rule")
	}
	if regexp.MustCompile(`animation\s*:`).MatchString(block) {
		t.Errorf(".note-card { ... } declares an unconditional animation: %s", block)
	}
}
