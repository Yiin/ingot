package composer

import (
	"testing"

	"github.com/Yiin/ingot/internal/ui/theme"
)

func TestPlaceholderText(t *testing.T) {
	tests := []struct{ project, want string }{
		{"Work", "Add a note or a prompt (work)"},
		{"MyProject", "Add a note or a prompt (myproject)"},
		{"", "Add a note or a prompt ()"},
	}
	for _, tt := range tests {
		if got := placeholderText(tt.project); got != tt.want {
			t.Errorf("placeholderText(%q) = %q, want %q", tt.project, got, tt.want)
		}
	}
}

// TestTargetHeight pins the acceptance criteria literally: 58dp flat
// through 0-3 lines, +18dp on the 4th, monotonic growth after that, and a
// clamp at maxLines so the composer never grows unbounded.
func TestTargetHeight(t *testing.T) {
	for lines := 0; lines <= minLines; lines++ {
		if got := targetHeight(lines); got != theme.ComposerMinHeight {
			t.Errorf("targetHeight(%d) = %d, want %d (flat through minLines)", lines, got, theme.ComposerMinHeight)
		}
	}

	if got, want := targetHeight(minLines+1), theme.ComposerMinHeight+theme.LineBody; got != want {
		t.Errorf("targetHeight(%d) = %d, want %d (+%ddp on the 4th line)", minLines+1, got, want, theme.LineBody)
	}

	capped := targetHeight(maxLines)
	if got := targetHeight(maxLines + 5); got != capped {
		t.Errorf("targetHeight(%d) = %d, want it clamped at %d", maxLines+5, got, capped)
	}
}
