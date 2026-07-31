package clipfmt

import (
	"strings"
	"testing"

	"github.com/Yiin/ingot/internal/store"
)

func notesOf(bodies ...string) []store.Note {
	notes := make([]store.Note, len(bodies))
	for i, b := range bodies {
		notes[i] = store.Note{Body: b}
	}
	return notes
}

// TestCopyAsListGoldenDemoOutput reproduces the exact demo paste at
// frame 41.8s from the child issue's acceptance criteria.
func TestCopyAsListGoldenDemoOutput(t *testing.T) {
	notes := notesOf(
		"How should configuration migrations work?",
		"Should plugins own their configuration schema?",
	)
	want := "1. How should configuration migrations work?\n2. Should plugins own their configuration schema?"
	if got := CopyAsList(notes); got != want {
		t.Errorf("CopyAsList = %q, want %q", got, want)
	}
}

func TestCopyAsListSingleNote(t *testing.T) {
	notes := notesOf("Only one thing to do")
	want := "1. Only one thing to do"
	if got := CopyAsList(notes); got != want {
		t.Errorf("CopyAsList(single) = %q, want %q", got, want)
	}
}

func TestCopyAsListEmpty(t *testing.T) {
	if got := CopyAsList(nil); got != "" {
		t.Errorf("CopyAsList(nil) = %q, want empty", got)
	}
}

func TestCopyAsListNoTrailingNewline(t *testing.T) {
	got := CopyAsList(notesOf("a", "b"))
	if strings.HasSuffix(got, "\n") {
		t.Errorf("CopyAsList = %q, has a trailing newline", got)
	}
}

func TestCopyAsListKeepsMarkdown(t *testing.T) {
	notes := notesOf("**bold** and `code`")
	want := "1. **bold** and `code`"
	if got := CopyAsList(notes); got != want {
		t.Errorf("CopyAsList = %q, want %q (Markdown kept verbatim)", got, want)
	}
}

func TestCopyAsListIndentsContinuationLines(t *testing.T) {
	notes := notesOf("first line\nsecond line")
	want := "1. first line\n   second line"
	if got := CopyAsList(notes); got != want {
		t.Errorf("CopyAsList = %q, want %q", got, want)
	}
}

func TestCopyAsListIndentWidthGrowsWithPrefix(t *testing.T) {
	bodies := make([]string, 10)
	for i := range bodies {
		bodies[i] = "line"
	}
	bodies[9] = "line one\nline two"
	notes := notesOf(bodies...)
	got := CopyAsList(notes)
	lines := strings.Split(got, "\n")
	// Entry 10 is "10. line one" then a continuation indented by 4
	// spaces (len("10. ") == 4).
	if lines[9] != "10. line one" {
		t.Fatalf("lines[9] = %q, want %q", lines[9], "10. line one")
	}
	if lines[10] != "    line two" {
		t.Errorf("continuation = %q, want 4-space indent", lines[10])
	}
}

func TestCopyAsListNeverIncludesDoneStateOrSectionTitles(t *testing.T) {
	notes := []store.Note{
		{Body: "task one", Done: true},
		{Body: "task two", Done: false},
	}
	got := CopyAsList(notes)
	want := "1. task one\n2. task two"
	if got != want {
		t.Errorf("CopyAsList = %q, want %q (no done markers, no titles)", got, want)
	}
}

func TestCopyJoinsRawBodiesWithBlankLine(t *testing.T) {
	notes := notesOf("first note", "second note")
	want := "first note\n\nsecond note"
	if got := Copy(notes); got != want {
		t.Errorf("Copy = %q, want %q", got, want)
	}
}

func TestCopyNoNumberingNoPrefix(t *testing.T) {
	notes := notesOf("**bold**")
	want := "**bold**"
	if got := Copy(notes); got != want {
		t.Errorf("Copy = %q, want %q", got, want)
	}
}

func TestCopyEmpty(t *testing.T) {
	if got := Copy(nil); got != "" {
		t.Errorf("Copy(nil) = %q, want empty", got)
	}
}

func TestHasMarkup(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"plain", "buy milk and eggs", false},
		{"bold", "use **TOML**", true},
		{"italic", "an _idea_", true},
		{"code span", "run `go test`", true},
		{"link", "[docs](https://example.com)", true},
		{"heading", "# Title", true},
		{"list", "- one\n- two", true},
		{"ordered list output of CopyAsList", "1. first\n2. second", true},
		{"stray asterisk not markup", "5 * 3 * 2 = 30", false},
		{"snake case not markup", "snake_case_name", false},
		{"blockquote", "> quoted", true},
		{"code block", "```\ncode\n```", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasMarkup(tt.s); got != tt.want {
				t.Errorf("HasMarkup(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}
