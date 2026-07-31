package toast

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/layershell"
	"github.com/Yiin/ingot/internal/ui/theme"
)

// hudNamespace is the dark HUD's own layer-shell namespace, distinct from
// the panel's "ingot-panel" so a user can write a layerrule exception for
// just the HUD (e.g. to keep it out of a screen recording) without also
// matching the panel.
const hudNamespace = "ingot-toast"

// HUD is the dark global toast: opaque near-black, white Semibold text,
// no icon, screen-centred ~195dp above the output bottom. It fires even
// when the panel is hidden or unfocused, so it owns a short-lived
// layer-shell surface of its own rather than living inside the panel's —
// built from internal/layershell's primitives directly rather than
// layershell.Panel, which is sized and right-edge-anchored for the
// always-present docked panel, a completely different policy than this
// short-lived, centred, keyboard-transparent surface.
//
// Verified: with keyboard-mode NONE, a sink window kept receiving
// keystrokes before, during, and after the HUD's surface was mapped
// above it — a GtkPopover cannot do this, it cannot leave the panel's
// own surface.
type HUD struct {
	win   *gtk.Window
	box   *gtk.Box
	label *gtk.Label
	seq   *sequencer

	// hideSrc is exit's own fade-out-then-unmap timer. It must be
	// tracked and canceled by enter, not just fire-and-forgotten: a Show
	// that lands mid-fade-out is a fresh show as far as sequencer is
	// concerned (see sequencer.go), but this raw timer from the previous
	// cycle would otherwise still fire afterwards and unmap the surface
	// out from under the toast it just re-showed.
	hideSrc glib.SourceHandle
}

// NewHUD builds the dark HUD's own layer-shell surface, unmapped until
// the first Show. It returns an error when the compositor does not
// support wlr-layer-shell — New (toast.go) falls back to
// org.freedesktop.Notifications in that case, per the epic's fallback
// policy: layer-shell is never optional-with-a-silent-downgrade here,
// callers must know which backend they got.
func NewHUD() (*HUD, error) {
	if !layershell.IsSupported() {
		return nil, fmt.Errorf("toast: compositor does not support wlr-layer-shell")
	}

	win := gtk.NewWindow()
	win.SetDecorated(false)
	win.SetResizable(false)

	label := newToastLabel()
	box := gtk.NewBox(gtk.OrientationHorizontal, theme.ToastIconGap)
	box.AddCSSClass("toast-dark")
	box.SetHAlign(gtk.AlignCenter)
	box.SetVAlign(gtk.AlignCenter)
	box.Append(label)
	win.SetChild(box)

	layershell.InitForWindow(win)
	layershell.SetNamespace(win, hudNamespace)
	layershell.SetLayer(win, layershell.LayerOverlay)
	layershell.SetAnchor(win, layershell.EdgeBottom, true)
	// No left/right anchor: the wlr-layer-shell convention centres an
	// axis with neither edge anchored, putting the HUD's centre x on the
	// output centre with no extra math — measured centre x = 961 on a
	// 1920-wide frame at both 11.0s and 13.77s.
	layershell.SetMargin(win, layershell.EdgeBottom, theme.HUDMarginBottom)
	layershell.SetExclusiveZone(win, 0) // never reserve space or push other surfaces
	layershell.SetKeyboardMode(win, layershell.KeyboardModeNone)

	h := &HUD{win: win, box: box, label: label}
	h.seq = newSequencer(glibScheduler{}, h.enter, h.holdReset, h.exit)
	return h, nil
}

// Show maps the surface with text, playing the 140ms scale/fade-in on
// first show and resetting the 1200ms hold on every call — see
// sequencer's doc comment for the never-stack replace semantics.
func (h *HUD) Show(text string) {
	h.label.SetText(newlineCollapser.Replace(text))
	h.seq.show()
}

func (h *HUD) enter() {
	if h.hideSrc != 0 {
		glib.SourceRemove(h.hideSrc)
		h.hideSrc = 0
	}
	h.box.RemoveCSSClass("toast-out")
	h.win.SetVisible(true)
	h.box.AddCSSClass("toast-in")
	glib.TimeoutAdd(uint(FadeInDuration.Milliseconds()), func() bool {
		h.box.RemoveCSSClass("toast-in")
		return false
	})
}

func (h *HUD) holdReset() {
	h.box.RemoveCSSClass("toast-out")
}

func (h *HUD) exit() {
	h.box.AddCSSClass("toast-out")
	h.hideSrc = glib.TimeoutAdd(uint(FadeOutDuration.Milliseconds()), func() bool {
		h.win.SetVisible(false)
		h.box.RemoveCSSClass("toast-out")
		h.hideSrc = 0
		return false
	})
}

var _ hudShower = (*HUD)(nil)
