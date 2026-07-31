package toast

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// InPanel is the light in-panel toast: translucent vibrancy fill (the
// focus ring and text of the card behind it show through — measured at
// frame 40.5s), near-black Semibold text, a filled black circle with a
// white tick 8dp before the text, panel-centred ~20dp above the
// composer. It fires only while the panel is visible — Copy as List is
// the spec's only measured instance.
//
// It is a GtkRevealer (RevealerTransitionTypeCrossfade) meant to be
// added as a GtkOverlay child of the panel; the crossfade handles the
// opacity half of the fade, style.css's .toast-in/.toast-out classes on
// the child box handle the scale/translateY half neither GtkRevealer nor
// its transition types can express.
type InPanel struct {
	revealer *gtk.Revealer
	box      *gtk.Box
	label    *gtk.Label
	seq      *sequencer

	// hideSrc is exit's own toast-out-class-removal timer. Tracked and
	// canceled by enter for the same reason as HUD.hideSrc: a Show
	// landing mid-fade-out must not let the previous cycle's leftover
	// timer touch state after the toast has already been re-shown.
	hideSrc glib.SourceHandle
}

// NewInPanel builds the light toast, not yet revealed, with its default
// bottom inset (theme.PanelToastGap — the composer's live height is not
// known here). The panel assembler (copper-l2z.26) should embed Widget()
// into the panel's GtkOverlay and call SetBottomInset with the
// composer's live height plus theme.PanelToastGap once that widget
// exists, so the toast tracks the composer as it grows.
func NewInPanel() *InPanel {
	label := newToastLabel()
	icon := newCheckIcon()

	box := gtk.NewBox(gtk.OrientationHorizontal, theme.ToastIconGap)
	box.AddCSSClass("toast-light")
	box.Append(icon)
	box.Append(label)

	revealer := gtk.NewRevealer()
	revealer.SetChild(box)
	revealer.SetTransitionType(gtk.RevealerTransitionTypeCrossfade)
	revealer.SetHAlign(gtk.AlignCenter)
	revealer.SetVAlign(gtk.AlignEnd)
	revealer.SetMarginBottom(theme.PanelToastGap)
	revealer.SetRevealChild(false)

	p := &InPanel{revealer: revealer, box: box, label: label}
	p.seq = newSequencer(glibScheduler{}, p.enter, p.holdReset, p.exit)
	return p
}

// Widget returns the revealer to embed as a GtkOverlay child.
func (p *InPanel) Widget() *gtk.Revealer { return p.revealer }

// SetBottomInset updates the toast's clearance above the panel's bottom
// edge, in dp. Call with the composer's live height plus
// theme.PanelToastGap whenever the composer grows or shrinks.
func (p *InPanel) SetBottomInset(dp int) {
	p.revealer.SetMarginBottom(dp)
}

// Show reveals the toast with text, playing the 140ms fade-in on first
// show and resetting the 1200ms hold on every call — see sequencer's
// doc comment for the never-stack replace semantics.
func (p *InPanel) Show(text string) {
	p.label.SetText(newlineCollapser.Replace(text))
	p.seq.show()
}

func (p *InPanel) enter() {
	if p.hideSrc != 0 {
		glib.SourceRemove(p.hideSrc)
		p.hideSrc = 0
	}
	p.box.RemoveCSSClass("toast-out")
	p.revealer.SetTransitionDuration(uint(FadeInDuration.Milliseconds()))
	p.revealer.SetRevealChild(true)
	p.box.AddCSSClass("toast-in")
	glib.TimeoutAdd(uint(FadeInDuration.Milliseconds()), func() bool {
		p.box.RemoveCSSClass("toast-in")
		return false
	})
}

func (p *InPanel) holdReset() {
	p.box.RemoveCSSClass("toast-out")
}

// exit starts the crossfade (SetRevealChild(false)) and the
// translateY(0->4px) keyframe (the "toast-out" class) at the same time,
// both timed to FadeOutDuration, rather than playing the keyframe first
// and only starting the crossfade once it finishes — sequentially, that
// would both double the visible exit to 240ms and, since the keyframe's
// own opacity animation ends before the crossfade's fill-mode holds it,
// produce a one-frame opacity pop back to full before the crossfade even
// starts.
func (p *InPanel) exit() {
	p.box.AddCSSClass("toast-out")
	p.revealer.SetTransitionDuration(uint(FadeOutDuration.Milliseconds()))
	p.revealer.SetRevealChild(false)
	p.hideSrc = glib.TimeoutAdd(uint(FadeOutDuration.Milliseconds()), func() bool {
		p.box.RemoveCSSClass("toast-out")
		p.hideSrc = 0
		return false
	})
}
