package widget

import (
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// strikeDuration is the done strikethrough's left-to-right wipe, timed to
// finish alongside the checkbox's own tick sweep (checkDuration, in
// easing.go).
const strikeDuration = 200 * time.Millisecond

// textGap is the gap between the checkbox's right edge and the text
// column that lands the column at the spec's 42dp from the card's left
// edge: 42 - card left padding - the checkbox's own width.
const textGap = 42 - theme.CardPadL - theme.CheckSize

// labelPageName and editorPageName are the two pages of Row's internal
// stack (see StartEdit in noterow_edit.go): the label's clamped/full
// markup, and — only while editing — a reused composer.Composer.
const (
	labelPageName  = "label"
	editorPageName = "editor"
)

// rowState captures the row's independent, combinable visual flags. Hover
// and keyboard focus are GTK's own :hover/:focus-visible pseudo-classes
// and need no tracking here; "selected wins over hover" falls out of
// style.css's declaration order, not from anything in this struct.
type rowState struct {
	selected bool
	anchor   bool // selection anchor within a multi-select; see cssClasses
	done     bool
	expanded bool
	dragging bool
}

// cssClasses returns the .note-card state classes for s. anchor only ever
// renders while selected: the original has no separate "focused within
// selection" affordance, so Ingot adds one (a 1dp inner ring, in
// style.css) only on the anchor row of a multi-selection.
func (s rowState) cssClasses() []string {
	classes := []string{"note-card"}
	if s.selected {
		classes = append(classes, "selected")
	}
	if s.selected && s.anchor {
		classes = append(classes, "selection-anchor")
	}
	if s.done {
		classes = append(classes, "done")
	}
	if s.expanded {
		classes = append(classes, "expanded")
	}
	if s.dragging {
		classes = append(classes, "dragging")
	}
	return classes
}

// Row is one note card: the theme's .note-card surface, a Checkbox
// vertically centred on the first text line (not the card), and a clamped
// Label whose text column starts at textGap past the checkbox so every
// wrapped line shares one left edge with no hanging indent.
type Row struct {
	*gtk.Box

	Checkbox *Checkbox
	Label    *Label

	strike *gtk.DrawingArea
	// stack holds the label overlay ("label", always present) and, only
	// while editing, a reused composer.Composer ("editor") — see
	// StartEdit in noterow_edit.go.
	stack *gtk.Stack
	state rowState

	strikeAnimating bool
	strikeStart     int64
	strikeElapsed   time.Duration
	strikeTickID    uint

	// applying is set while SetChecked is driving the checkbox
	// programmatically, so checkboxToggled does not treat a recycled list
	// item's reset-then-apply bind as a user click and replay the 200ms
	// strike wipe on every recycle.
	applying bool

	// editing is non-nil while StartEdit has swapped the label for an
	// inline composer.Composer — see noterow_edit.go.
	editing *editState
}

// NewRow assembles one note card, initially idle and unchecked. A click
// on the checkbox toggles both it and the row's own done state; anything
// else (selection, dragging, expansion) is the owning list's job to drive
// through the Set* methods below.
func NewRow() *Row {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.SetFocusable(true)
	box.SetCanFocus(true)

	checkbox := NewCheckbox()
	checkbox.SetVAlign(gtk.AlignStart)
	checkbox.SetMarginEnd(textGap)
	// Centres the 17dp circle on the 18dp first text line, not the card.
	checkbox.SetMarginTop((theme.LineBody - theme.CheckSize) / 2)

	label := NewLabel(3)
	label.SetHExpand(true)

	strike := gtk.NewDrawingArea()
	strike.SetCanTarget(false)

	overlay := gtk.NewOverlay()
	overlay.SetChild(label.Label)
	overlay.AddOverlay(strike)
	overlay.SetClipOverlay(strike, false)
	overlay.SetMeasureOverlay(strike, false)

	// Vhomogeneous/Hhomogeneous false + InterpolateSize false: the stack
	// must size itself to whichever page is actually visible, not the
	// largest of the two — GTK's default homogeneous sizing would
	// otherwise make every row as tall as an open inline editor even
	// while showing the plain label. TransitionTypeNone: the label <->
	// editor swap (StartEdit/endEdit, noterow_edit.go) is instant.
	stack := gtk.NewStack()
	stack.SetHExpand(true)
	stack.SetHhomogeneous(false)
	stack.SetVhomogeneous(false)
	stack.SetInterpolateSize(false)
	stack.SetTransitionType(gtk.StackTransitionTypeNone)
	stack.AddNamed(overlay, labelPageName)
	stack.SetVisibleChildName(labelPageName)

	box.Append(checkbox)
	box.Append(stack)

	r := &Row{Box: box, Checkbox: checkbox, Label: label, strike: strike, stack: stack}
	strike.SetDrawFunc(r.drawStrike)
	r.applyCSS()

	checkbox.ConnectToggled(r.checkboxToggled)

	return r
}

// rowOwnedClasses are exactly the classes applyCSS ever adds or removes.
var rowOwnedClasses = []string{"note-card", "selected", "selection-anchor", "done", "expanded", "dragging"}

// applyCSS toggles each of rowOwnedClasses independently (AddCSSClass /
// RemoveCSSClass), never via SetCSSClasses — a caller may have added a
// class this package doesn't own (e.g. internal/ui/notelist's
// "just-inserted" insert-animation class, on a Row it recycles across
// list items), and SetCSSClasses replaces the widget's entire class
// list, which would silently strip it on the next state repaint.
func (r *Row) applyCSS() {
	want := make(map[string]bool)
	for _, c := range r.state.cssClasses() {
		want[c] = true
	}
	for _, c := range rowOwnedClasses {
		if want[c] {
			r.AddCSSClass(c)
		} else {
			r.RemoveCSSClass(c)
		}
	}
}

// checkboxToggled mirrors the checkbox's state onto the row's own done
// state, animated, for a real user click. It is registered once in
// NewRow and does nothing while SetChecked is applying the state
// programmatically (see applying).
func (r *Row) checkboxToggled(checked bool) {
	if r.applying {
		return
	}
	r.SetDone(checked, true)
}

// SetChecked sets both the checkbox and the row's done state together,
// without replaying this Row's own checkboxToggled handler — safe to
// call on every bind of a recycled row. It does not suppress any other
// subscriber registered via Checkbox.ConnectToggled: a caller that binds
// its own toggle handler onto Row.Checkbox (as internal/ui/notelist
// does) must guard that handler itself while applying a programmatic
// state.
func (r *Row) SetChecked(checked bool, animate bool) {
	r.applying = true
	r.Checkbox.SetChecked(checked, animate)
	r.applying = false
	r.SetDone(checked, animate)
}

// SetDone toggles the .done state and, when turning on with animate,
// plays the strikethrough wipe.
func (r *Row) SetDone(done bool, animate bool) {
	if done == r.state.done {
		return
	}
	r.state.done = done
	r.applyCSS()
	if done && animate && enableAnimations() {
		r.startStrike()
	} else {
		r.stopStrike()
		r.strike.QueueDraw()
	}
}

// SetSelected toggles the .selected state (fill #EFF6FF, an accent ring
// drawn outside the card — see style.css).
func (r *Row) SetSelected(selected bool) {
	r.state.selected = selected
	r.applyCSS()
}

// SetSelectionAnchor marks r as the keyboard-focus anchor within a
// multi-selection.
func (r *Row) SetSelectionAnchor(anchor bool) {
	r.state.anchor = anchor
	r.applyCSS()
}

// IsExpanded reports whether the row's 3-line cap is currently lifted.
func (r *Row) IsExpanded() bool { return r.state.expanded }

// SetExpanded drops (or restores) the 3-line cap on the row's Label. A
// no-op if already in that state (bindRow's own unconditional reset
// relies on this to skip re-rendering an already-collapsed row on every
// recycle).
//
// This does not animate the row's own height, despite the child spec's
// 180ms figure: GTK never allocates a plain widget smaller than its
// children's own natural minimum size, so a SetSizeRequest-driven clip
// (the technique internal/ui/composer uses for its growing text view)
// only actually works for the collapse direction — expanding switches
// the label to its full, unclamped natural size first, which
// immediately becomes the row's own minimum and defeats the clip before
// a single frame ticks. A real height animation needs the label to sit
// inside a genuinely scrollable container (GtkScrolledWindow, the one
// container that can allocate less than its child's natural size) —
// out of scope for this row's toggle; left for whoever picks it up.
func (r *Row) SetExpanded(expanded bool) {
	if expanded == r.state.expanded {
		return
	}
	r.state.expanded = expanded
	if expanded {
		r.Label.Expand()
	} else {
		r.Label.Collapse()
	}
	r.applyCSS()
}

// SetDragging toggles the .dragging lift/scale/opacity treatment used
// while the row is being reordered.
func (r *Row) SetDragging(dragging bool) {
	r.state.dragging = dragging
	r.applyCSS()
}

func (r *Row) startStrike() {
	r.stopStrike()
	r.strikeAnimating = true
	r.strikeStart = -1
	r.strikeElapsed = 0
	r.strikeTickID = r.strike.AddTickCallback(r.onStrikeTick)
}

func (r *Row) stopStrike() {
	if r.strikeAnimating {
		r.strike.RemoveTickCallback(r.strikeTickID)
	}
	r.strikeAnimating = false
}

func (r *Row) onStrikeTick(_ gtk.Widgetter, frameClock gdk.FrameClocker) bool {
	now := gdk.BaseFrameClock(frameClock).FrameTime()
	if r.strikeStart < 0 {
		r.strikeStart = now
	}
	r.strikeElapsed = time.Duration(now-r.strikeStart) * time.Microsecond
	r.strike.QueueDraw()
	if r.strikeElapsed >= strikeDuration {
		r.strikeElapsed = strikeDuration
		r.strikeAnimating = false
		return false
	}
	return true
}

func (r *Row) strikeProgress() float64 {
	if !r.state.done {
		return 0
	}
	if !r.strikeAnimating {
		return 1
	}
	return clamp01(float64(r.strikeElapsed) / float64(strikeDuration))
}

// drawStrike paints the done strikethrough as a left-to-right wipe. It
// approximates the x-height mid-line as half the first line's height
// (theme.LineBody/2): getting the real Pango x-height needs a
// font-metrics query this overlay has no other reason to make.
func (r *Row) drawStrike(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
	reveal := r.strikeProgress()
	if reveal <= 0 {
		return
	}
	y := float64(theme.LineBody) / 2
	// Read at draw time so a colour-scheme change repaints in the new
	// palette — see theme.Colors.
	ir, ig, ib := theme.ParseRGB(theme.Colors().InkDone)

	cr.NewPath()
	cr.MoveTo(0, y)
	cr.LineTo(float64(width)*reveal, y)
	cr.SetLineWidth(1)
	cr.SetSourceRGB(ir, ig, ib)
	cr.Stroke()
}
