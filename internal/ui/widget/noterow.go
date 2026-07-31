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
	state  rowState

	strikeAnimating bool
	strikeStart     int64
	strikeElapsed   time.Duration
	strikeTickID    uint
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

	box.Append(checkbox)
	box.Append(overlay)

	r := &Row{Box: box, Checkbox: checkbox, Label: label, strike: strike}
	strike.SetDrawFunc(r.drawStrike)
	r.applyCSS()

	checkbox.ConnectToggled(func(checked bool) {
		r.SetDone(checked, true)
	})

	return r
}

func (r *Row) applyCSS() {
	r.SetCSSClasses(r.state.cssClasses())
}

// SetChecked sets both the checkbox and the row's done state together.
func (r *Row) SetChecked(checked bool, animate bool) {
	r.Checkbox.SetChecked(checked, animate)
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

// SetExpanded drops (or restores) the 3-line cap on the row's Label.
func (r *Row) SetExpanded(expanded bool) {
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
	ir, ig, ib := hexRGB(theme.InkDone)

	cr.NewPath()
	cr.MoveTo(0, y)
	cr.LineTo(float64(width)*reveal, y)
	cr.SetLineWidth(1)
	cr.SetSourceRGB(ir, ig, ib)
	cr.Stroke()
}
