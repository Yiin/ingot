package keymap

// EscapeTarget is the set of hooks HandleEscape's cascade checks and
// acts on. Each query method reports whether that cascade step
// currently applies; HandleEscape calls the first step whose query is
// true (ComposerFocused is inverted — see below) and stops there. The
// real panel implements this over its popover, inline editor, search
// entry, Nav, composer, and window; tests use a fake.
type EscapeTarget interface {
	// PopoverOpen is step 1: a context menu, overflow menu, or Move to
	// submenu is open.
	PopoverOpen() bool
	ClosePopover()

	// EditingInline is step 2: a note is being edited in place.
	EditingInline() bool
	CancelInlineEdit()

	// SearchHasText is step 3: the search field has text (whether or
	// not it currently holds focus).
	SearchHasText() bool
	ClearSearchText()

	// HasSelection is step 4: one or more notes are selected.
	HasSelection() bool
	ClearSelection()

	// ComposerFocused is step 5's guard, inverted: step 5 only applies
	// when the composer does *not* already have focus.
	ComposerFocused() bool
	FocusComposer()

	// HidePanel is step 6, the fallback once every earlier step is
	// inapplicable.
	HidePanel()
}

// HandleEscape runs the documented Escape cascade against t — close a
// popover or menu, cancel an inline edit, clear the search text, clear
// the selection, return focus to the composer, hide the panel — firing
// exactly the first step that applies, and reports which step that was.
func HandleEscape(t EscapeTarget) string {
	switch {
	case t.PopoverOpen():
		t.ClosePopover()
		return "close-popover"
	case t.EditingInline():
		t.CancelInlineEdit()
		return "cancel-inline-edit"
	case t.SearchHasText():
		t.ClearSearchText()
		return "clear-search-text"
	case t.HasSelection():
		t.ClearSelection()
		return "clear-selection"
	case !t.ComposerFocused():
		t.FocusComposer()
		return "focus-composer"
	default:
		t.HidePanel()
		return "hide-panel"
	}
}
