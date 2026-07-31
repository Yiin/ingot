// Package composer builds the panel's note/prompt composer: a
// GtkTextView-backed card, fixed at three lines tall, that grows past that
// only as far as it needs and no further than a cap. It must be a
// GtkTextView — GtkEntry cannot hold a newline.
package composer

import (
	"strings"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/motion"
	"github.com/Yiin/ingot/internal/ui/theme"
)

const (
	// minLines is the composer's resting height in text lines (0-3
	// paragraphs all render at theme.ComposerMinHeight, per spec).
	minLines = 3

	// maxLines caps how far the composer may grow. Not a measured design
	// token — the spec only says "clamping at max" — chosen so the
	// composer never eats more than roughly a third of the 640dp panel,
	// leaving room for the note list above it.
	maxLines = 10

	growthMs = 120
)

// targetHeight maps a paragraph count (gtk.TextBuffer.LineCount, i.e. hard
// breaks — Shift+Return is the only way to add one, since plain Return
// commits) to the composer's card height: flat at theme.ComposerMinHeight
// through minLines, then +theme.LineBody per line beyond it, clamped at
// maxLines. This is the acceptance criterion verbatim: 58dp at 0-3 lines,
// +18dp on the 4th.
func targetHeight(lines int) int {
	if lines < minLines {
		lines = minLines
	}
	if lines > maxLines {
		lines = maxLines
	}
	return theme.ComposerMinHeight + (lines-minLines)*theme.LineBody
}

// Composer is the auto-growing note/prompt input.
//
// It is deliberately reusable: copper-l2z.27 swaps a note row's label for
// one of these on Edit and swaps back on commit, using SetText/Text/Focus
// the same way the top-level composer uses them.
type Composer struct {
	root        *gtk.Box
	scroll      *gtk.ScrolledWindow
	view        *gtk.TextView
	buffer      *gtk.TextBuffer
	placeholder *gtk.Label

	project string

	// placeholderDisabled is DisablePlaceholder's own flag: the bottom
	// composer is genuinely idle-and-inviting when empty, but
	// copper-l2z.27's reused inline row editor is not — an empty buffer
	// there is a mid-edit accident (select all, delete), not a resting
	// state that should say "Add a note or a prompt ()".
	placeholderDisabled bool

	onCommit        func(text string)
	onHeightChanged func(height int)

	currentHeight int
	tickID        uint
	animStart     int
	animTarget    int
	animStartTime int64
}

// New builds a composer whose placeholder names project (lowercased).
func New(project string) *Composer {
	c := &Composer{project: project, currentHeight: targetHeight(0)}

	c.buffer = gtk.NewTextBuffer(nil)

	c.view = gtk.NewTextViewWithBuffer(c.buffer)
	c.view.SetWrapMode(gtk.WrapWordChar)
	c.view.SetAcceptsTab(false) // Tab should leave the composer
	c.view.AddCSSClass("composer-text")

	c.scroll = gtk.NewScrolledWindow()
	c.scroll.SetChild(c.view)
	c.scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	c.scroll.SetPropagateNaturalHeight(true)
	c.scroll.SetMinContentHeight(c.currentHeight)
	c.scroll.SetMaxContentHeight(targetHeight(maxLines)) // the cap

	c.placeholder = gtk.NewLabel(placeholderText(project))
	c.placeholder.AddCSSClass("composer-placeholder")
	c.placeholder.SetHAlign(gtk.AlignStart)
	c.placeholder.SetVAlign(gtk.AlignStart)
	c.placeholder.SetCanTarget(false) // let clicks fall through to the TextView

	overlay := gtk.NewOverlay()
	overlay.SetChild(c.scroll)
	overlay.AddOverlay(c.placeholder)

	c.root = gtk.NewBox(gtk.OrientationVertical, 0)
	c.root.AddCSSClass("composer")
	c.root.Append(overlay)

	c.buffer.ConnectChanged(func() { c.handleTextChanged() })

	c.installKeyHandling()
	c.installFocusRing()

	return c
}

// placeholderText renders "Add a note or a prompt (<project>)" with the
// project name lowercased.
func placeholderText(project string) string {
	return "Add a note or a prompt (" + strings.ToLower(project) + ")"
}

// Widget returns the root widget to place in the panel.
func (c *Composer) Widget() gtk.Widgetter { return c.root }

// SetProject updates the placeholder for a newly active project.
func (c *Composer) SetProject(project string) {
	c.project = project
	c.placeholder.SetText(placeholderText(project))
}

// Text returns the current, untrimmed buffer contents.
func (c *Composer) Text() string {
	return c.buffer.Text(c.buffer.StartIter(), c.buffer.EndIter(), false)
}

// SetText replaces the buffer contents, e.g. to preload a note being
// edited inline (copper-l2z.27).
func (c *Composer) SetText(text string) { c.buffer.SetText(text) }

// Focus grabs keyboard focus into the text view.
func (c *Composer) Focus() { c.view.GrabFocus() }

// View exposes the underlying GtkTextView. The top-level composer has
// no need for this itself; it exists for copper-l2z.27's inline row
// editing, which reuses Composer wholesale but still needs to attach
// its own Escape-cancels key controller on top of it.
func (c *Composer) View() *gtk.TextView { return c.view }

// DisablePlaceholder permanently hides the placeholder for this
// instance, regardless of buffer emptiness — see placeholderDisabled.
func (c *Composer) DisablePlaceholder() {
	c.placeholderDisabled = true
	c.placeholder.SetVisible(false)
}

// OnCommit registers f to be called with the trimmed text every time the
// composer commits (plain Enter or Ctrl+Enter).
func (c *Composer) OnCommit(f func(text string)) { c.onCommit = f }

// OnHeightChanged registers f to be called with the composer's current
// card height every time it changes (including mid-animation, every
// tick) — the panel (copper-l2z.26) uses this to keep the in-panel
// toast's bottom inset tracking the composer as it grows.
func (c *Composer) OnHeightChanged(f func(height int)) { c.onHeightChanged = f }

func (c *Composer) handleTextChanged() {
	if !c.placeholderDisabled {
		empty := c.buffer.CharCount() == 0
		c.placeholder.SetVisible(empty)
	}
	c.animateHeightTo(targetHeight(c.buffer.LineCount()))
}

func (c *Composer) animateHeightTo(target int) {
	if target == c.currentHeight {
		return
	}
	if c.tickID != 0 {
		c.view.RemoveTickCallback(c.tickID)
		c.tickID = 0
	}

	// This growth is a natural-size change (SetMinContentHeight), not a
	// CSS property, so it is driven by hand via AddTickCallback below —
	// unlike a CSS transition, it does not honour gtk-enable-animations
	// for free and must check motion.EnableAnimations itself, the same
	// way internal/ui/widget's checkbox/strikethrough already do.
	if !motion.EnableAnimations() {
		c.setContentHeight(target)
		return
	}

	c.animStart = c.currentHeight
	c.animTarget = target
	c.animStartTime = -1

	c.tickID = c.view.AddTickCallback(func(_ gtk.Widgetter, frameClock gdk.FrameClocker) bool {
		now := gdk.BaseFrameClock(frameClock).FrameTime()
		if c.animStartTime < 0 {
			c.animStartTime = now
		}

		elapsedMs := float64(now-c.animStartTime) / 1000
		t := elapsedMs / growthMs
		if t >= 1 {
			c.setContentHeight(c.animTarget)
			c.tickID = 0
			return false
		}

		eased := 1 - (1-t)*(1-t)*(1-t) // ease-out cubic
		h := c.animStart + int(float64(c.animTarget-c.animStart)*eased)
		c.setContentHeight(h)
		return true
	})
}

func (c *Composer) setContentHeight(h int) {
	c.currentHeight = h
	c.scroll.SetMinContentHeight(h)
	if c.onHeightChanged != nil {
		c.onHeightChanged(h)
	}
}

// installKeyHandling wires Return/KP_Enter/ISO_Enter. It needs the
// capture phase — without it the TextView eats Return first and inserts a
// newline before any bubble-phase handler sees the event.
func (c *Composer) installKeyHandling() {
	ctrl := gtk.NewEventControllerKey()
	ctrl.SetPropagationPhase(gtk.PhaseCapture)
	ctrl.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		switch keyval {
		case gdk.KEY_Return, gdk.KEY_KP_Enter, gdk.KEY_ISO_Enter:
		default:
			return false
		}

		if state&gdk.ShiftMask != 0 {
			return false // let the TextView insert the newline
		}

		c.tryCommit(state&gdk.ControlMask == 0)
		return true
	})
	c.view.AddController(ctrl)
}

func (c *Composer) tryCommit(clearOnCommit bool) {
	trimmed := strings.TrimSpace(c.Text())
	if trimmed == "" {
		c.flashReject()
		return
	}

	if c.onCommit != nil {
		c.onCommit(trimmed)
	}
	if clearOnCommit {
		c.buffer.SetText("")
	}
}

// flashReject gives whitespace-only commit attempts a 200ms reddish ring,
// keeping the text.
func (c *Composer) flashReject() {
	c.root.AddCSSClass("reject")
	glib.TimeoutAdd(200, func() bool {
		c.root.RemoveCSSClass("reject")
		return false
	})
}

func (c *Composer) installFocusRing() {
	focus := gtk.NewEventControllerFocus()
	focus.ConnectEnter(func() { c.root.AddCSSClass("focused") })
	focus.ConnectLeave(func() { c.root.RemoveCSSClass("focused") })
	c.view.AddController(focus)
}
