package widget

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/composer"
)

// editState is the transient state for one inline-edit session: a
// fresh composer.Composer built by StartEdit and thrown away by
// endEdit, on either commit or cancel. Row never keeps more than one
// alive at a time (StartEdit is a no-op while already editing).
type editState struct {
	composer *composer.Composer
}

// IsEditing reports whether the row is currently showing its inline
// editor in place of its label.
func (r *Row) IsEditing() bool { return r.editing != nil }

// StartEdit swaps the row's label for a reused composer.Composer — see
// composer.go's own doc comment, which names this exact call site —
// seeded with raw, the note's raw Markdown source (never the rendered
// text), caret at the end, styled identically to the bottom composer's
// own focused state (composer.Composer already carries that CSS class
// on focus; nothing extra is needed here). Shift+Enter inserts a
// newline and plain Enter commits, both the composer's own existing key
// handling. Escape cancels: since the row's Label is never touched
// while editing, "restoring the original text and its rendered
// attributes" falls out for free by simply showing the label again. A
// no-op if already editing.
func (r *Row) StartEdit(raw string, onCommit func(text string)) {
	if r.editing != nil {
		return
	}

	comp := composer.New("")
	comp.DisablePlaceholder()
	comp.SetText(raw)

	r.editing = &editState{composer: comp}

	comp.OnCommit(func(text string) {
		r.endEdit()
		if onCommit != nil {
			onCommit(text)
		}
	})

	// The bottom composer never cancels, so Escape is wired here, on
	// this specific instance, rather than inside the composer package.
	// PhaseCapture: without it the TextView's own IM context can eat
	// Escape before this ever sees it (the same reason composer.go's own
	// Return handling needs capture phase).
	esc := gtk.NewEventControllerKey()
	esc.SetPropagationPhase(gtk.PhaseCapture)
	esc.ConnectKeyPressed(func(keyval, _ uint, _ gdk.ModifierType) bool {
		if keyval != gdk.KEY_Escape {
			return false
		}
		r.CancelEdit()
		return true
	})
	comp.View().AddController(esc)

	r.stack.AddNamed(comp.Widget(), editorPageName)
	r.stack.SetVisibleChildName(editorPageName)
	comp.Focus()
	comp.View().Buffer().PlaceCursor(comp.View().Buffer().EndIter())
}

// CancelEdit exits inline-edit mode without invoking the commit
// callback, discarding whatever was typed and restoring the label
// exactly as it read before StartEdit. A no-op if not currently
// editing.
func (r *Row) CancelEdit() {
	if r.editing == nil {
		return
	}
	r.endEdit()
}

// endEdit is StartEdit's teardown, shared by both the commit path (via
// comp.OnCommit) and CancelEdit: swap back to the label page, drop the
// editor's composer.Composer so the next StartEdit builds a fresh one,
// and return keyboard focus to the row itself — without this, removing
// the focused TextView leaves nothing focused at all, and Enter/Right/
// Left stop reaching the row until something else is clicked.
func (r *Row) endEdit() {
	es := r.editing
	if es == nil {
		return
	}
	r.editing = nil
	r.stack.SetVisibleChildName(labelPageName)
	r.stack.Remove(es.composer.Widget())
	r.GrabFocus()
}
