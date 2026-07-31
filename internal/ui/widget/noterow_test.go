package widget

import (
	"slices"
	"testing"
)

func TestRowStateCSSClasses(t *testing.T) {
	cases := []struct {
		name  string
		state rowState
		want  []string
	}{
		{"idle", rowState{}, []string{"note-card"}},
		{"selected", rowState{selected: true}, []string{"note-card", "selected"}},
		{"anchor without selection is not shown", rowState{anchor: true}, []string{"note-card"}},
		{
			"multi-selected anchor",
			rowState{selected: true, anchor: true},
			[]string{"note-card", "selected", "selection-anchor"},
		},
		{"done", rowState{done: true}, []string{"note-card", "done"}},
		{
			"done and selected",
			rowState{done: true, selected: true},
			[]string{"note-card", "selected", "done"},
		},
		{"expanded", rowState{expanded: true}, []string{"note-card", "expanded"}},
		{"dragged", rowState{dragging: true}, []string{"note-card", "dragging"}},
		{
			"every state combined",
			rowState{selected: true, anchor: true, done: true, expanded: true, dragging: true},
			[]string{"note-card", "selected", "selection-anchor", "done", "expanded", "dragging"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.state.cssClasses()
			if !slices.Equal(got, c.want) {
				t.Errorf("cssClasses() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestStrikeProgress(t *testing.T) {
	r := &Row{}
	if got := r.strikeProgress(); got != 0 {
		t.Errorf("strikeProgress() on an idle, not-done row = %v, want 0", got)
	}

	r.state.done = true
	if got := r.strikeProgress(); got != 1 {
		t.Errorf("strikeProgress() when done and not animating = %v, want 1", got)
	}

	r.strikeAnimating = true
	r.strikeElapsed = strikeDuration / 2
	if got := r.strikeProgress(); got != 0.5 {
		t.Errorf("strikeProgress() mid-animation = %v, want 0.5", got)
	}
}

func TestTextGapLandsColumnAt42dp(t *testing.T) {
	const cardPadL = 12
	const checkSize = 17
	if got := cardPadL + checkSize + textGap; got != 42 {
		t.Errorf("card-pad-l + checkbox width + textGap = %d, want 42", got)
	}
}

// TestCheckboxToggledSuppressedWhileApplying guards against the strike
// animation replaying on every recycled-row bind: SetChecked drives the
// checkbox programmatically, and checkboxToggled (the click handler) must
// not treat that as a user click while applying is set.
func TestCheckboxToggledSuppressedWhileApplying(t *testing.T) {
	r := &Row{applying: true}
	r.checkboxToggled(true) // must not touch r.state or any widget
	if r.state.done {
		t.Errorf("checkboxToggled set done=true while applying, want no-op")
	}
}
