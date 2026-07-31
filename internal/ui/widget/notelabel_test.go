package widget

import (
	"strings"
	"testing"

	"github.com/Yiin/ingot/internal/ui/mdpango"
)

func TestClampedMarkupTrimsTrailingWhitespace(t *testing.T) {
	got := clampedMarkup("captured text\n\n\n")
	want := mdpango.SafeCollapsed("captured text")
	if got != want {
		t.Errorf("clampedMarkup with trailing blank lines = %q, want %q", got, want)
	}
}

func TestClampedMarkupNeverContainsNewline(t *testing.T) {
	bodies := []string{
		"line one\nline two",
		"- one\n- two\n- three",
		"para one\n\npara two\n\npara three",
		"```\ncode line one\ncode line two\n```",
	}
	for _, body := range bodies {
		if got := clampedMarkup(body); strings.Contains(got, "\n") {
			t.Errorf("clampedMarkup(%q) = %q, contains a newline", body, got)
		}
	}
}

func TestClampedMarkupPreservesInlineFormatting(t *testing.T) {
	got := clampedMarkup("**bold**")
	want := "<b>bold</b>"
	if got != want {
		t.Errorf("clampedMarkup(%q) = %q, want %q", "**bold**", got, want)
	}
}
