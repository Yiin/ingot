package motion

import "time"

// revealable is any widget with GtkRevealer's own transition-duration and
// reveal-child API — *gtk.Revealer satisfies it directly.
type revealable interface {
	SetTransitionDuration(msec uint)
	SetRevealChild(revealChild bool)
}

// Reveal shows or hides r, using showDuration as the transition length
// going to visible and hideDuration going to hidden — GtkRevealer only
// has one transition-duration property, so it must be set to the
// direction-appropriate value immediately before each call, per the
// panel show/hide spec's own asymmetric 200ms/150ms timing.
//
// Unlike Animate and FlashClass, Reveal does not itself call
// EnableAnimations: GtkRevealer's transition is a built-in GTK
// animation, not a hand-rolled one, so it already honours
// gtk-enable-animations for free.
func Reveal(r revealable, show bool, showDuration, hideDuration time.Duration) {
	if show {
		r.SetTransitionDuration(uint(showDuration.Milliseconds()))
	} else {
		r.SetTransitionDuration(uint(hideDuration.Milliseconds()))
	}
	r.SetRevealChild(show)
}
