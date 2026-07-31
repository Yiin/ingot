package keymap

import "testing"

// fakeEscapeTarget is a fully-controllable EscapeTarget: each test sets
// exactly the flags relevant to the step under test and checks which Do
// method HandleEscape called.
type fakeEscapeTarget struct {
	popoverOpen, editingInline, searchHasText, hasSelection, composerFocused bool

	closedPopover, canceledEdit, clearedSearch, clearedSelection, focusedComposer, hidPanel bool
}

func (f *fakeEscapeTarget) PopoverOpen() bool     { return f.popoverOpen }
func (f *fakeEscapeTarget) ClosePopover()         { f.closedPopover = true }
func (f *fakeEscapeTarget) EditingInline() bool   { return f.editingInline }
func (f *fakeEscapeTarget) CancelInlineEdit()     { f.canceledEdit = true }
func (f *fakeEscapeTarget) SearchHasText() bool   { return f.searchHasText }
func (f *fakeEscapeTarget) ClearSearchText()      { f.clearedSearch = true }
func (f *fakeEscapeTarget) HasSelection() bool    { return f.hasSelection }
func (f *fakeEscapeTarget) ClearSelection()       { f.clearedSelection = true }
func (f *fakeEscapeTarget) ComposerFocused() bool { return f.composerFocused }
func (f *fakeEscapeTarget) FocusComposer()        { f.focusedComposer = true }
func (f *fakeEscapeTarget) HidePanel()            { f.hidPanel = true }

func TestEscapeCascadeOrder(t *testing.T) {
	cases := []struct {
		name string
		set  func(f *fakeEscapeTarget)
		want string
		did  func(f *fakeEscapeTarget) bool
	}{
		{
			"step 1: popover open beats everything else",
			func(f *fakeEscapeTarget) {
				f.popoverOpen = true
				f.editingInline = true
				f.searchHasText = true
				f.hasSelection = true
				f.composerFocused = false
			},
			"close-popover",
			func(f *fakeEscapeTarget) bool { return f.closedPopover },
		},
		{
			"step 2: inline edit beats search/selection/focus",
			func(f *fakeEscapeTarget) {
				f.editingInline = true
				f.searchHasText = true
				f.hasSelection = true
				f.composerFocused = false
			},
			"cancel-inline-edit",
			func(f *fakeEscapeTarget) bool { return f.canceledEdit },
		},
		{
			"step 3: search text beats selection/focus",
			func(f *fakeEscapeTarget) {
				f.searchHasText = true
				f.hasSelection = true
				f.composerFocused = false
			},
			"clear-search-text",
			func(f *fakeEscapeTarget) bool { return f.clearedSearch },
		},
		{
			"step 4: selection beats returning focus to the composer",
			func(f *fakeEscapeTarget) {
				f.hasSelection = true
				f.composerFocused = false
			},
			"clear-selection",
			func(f *fakeEscapeTarget) bool { return f.clearedSelection },
		},
		{
			"step 5: composer not focused, nothing else applies",
			func(f *fakeEscapeTarget) {
				f.composerFocused = false
			},
			"focus-composer",
			func(f *fakeEscapeTarget) bool { return f.focusedComposer },
		},
		{
			"step 6: composer already focused, nothing else applies -> hide panel",
			func(f *fakeEscapeTarget) {
				f.composerFocused = true
			},
			"hide-panel",
			func(f *fakeEscapeTarget) bool { return f.hidPanel },
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeEscapeTarget{}
			c.set(f)
			if got := HandleEscape(f); got != c.want {
				t.Errorf("HandleEscape() = %q, want %q", got, c.want)
			}
			if !c.did(f) {
				t.Errorf("HandleEscape() did not run the %q step's action", c.want)
			}
		})
	}
}
