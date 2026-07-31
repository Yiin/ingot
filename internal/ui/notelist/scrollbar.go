package notelist

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// scrollbarHold is how long the overlay scrollbar stays fully visible
// after the last scroll before its 300ms CSS fade-out begins (style.css's
// .ingot-scrollbar / .ingot-scrollbar.scrolling pair does the actual
// fade; this only toggles the class).
const scrollbarHold = 700

// overlayScrollbar is the ~5dp indicator drawn over the list viewport,
// inset 3dp from the panel edge, spanning only the scrolled window (never
// the composer below it) because it is an overlay child of that same
// gtk.ScrolledWindow, not of the whole panel.
type overlayScrollbar struct {
	*gtk.Scrollbar

	fadeSrc glib.SourceHandle
}

func newOverlayScrollbar(adjustment *gtk.Adjustment) *overlayScrollbar {
	sb := gtk.NewScrollbar(gtk.OrientationVertical, adjustment)
	sb.AddCSSClass("ingot-scrollbar")
	sb.SetHAlign(gtk.AlignEnd)
	sb.SetVAlign(gtk.AlignFill)
	sb.SetCanTarget(false) // pure indicator: no phantom hit strip while faded out

	return &overlayScrollbar{Scrollbar: sb}
}

// poke shows the indicator (CSS fades it in over 80ms), holding it for
// scrollbarHold before fading it back out over 300ms — matching the
// original's overlay-scrollbar timing (demo frame 30.7s). It is a no-op
// while there is nothing to scroll.
func (s *overlayScrollbar) poke(adjustment *gtk.Adjustment) {
	if adjustment.Upper()-adjustment.Lower() <= adjustment.PageSize() {
		return
	}
	s.AddCSSClass("scrolling")
	if s.fadeSrc != 0 {
		glib.SourceRemove(s.fadeSrc)
	}
	s.fadeSrc = glib.TimeoutAdd(scrollbarHold, func() bool {
		s.RemoveCSSClass("scrolling")
		s.fadeSrc = 0
		return false
	})
}
