package motion

// Easing maps elapsed progress t in [0,1] to eased progress in [0,1]. It
// is the shape a hand-rolled AddTickCallback animation (tick.go's
// Animate) samples every frame; a CSS-driven one instead names the same
// curve directly in style.css, and the two are kept in sync by this
// package's own naming, not by sharing code — GTK's CSS engine has no Go
// entry point to call into.
type Easing func(t float64) float64

// CubicBezier returns the easing function for the CSS
// cubic-bezier(x1, y1, x2, y2) timing curve anchored at (0,0) and (1,1).
// It solves for the curve parameter u at x(u) == t by bisection, then
// evaluates y(u) — the same construction internal/ui/widget's own
// private cubicBezier uses (that package predates this one and is left
// alone rather than migrated, to avoid touching its own tested easing
// values).
func CubicBezier(x1, y1, x2, y2 float64) Easing {
	component := func(p1, p2 float64) func(u float64) float64 {
		return func(u float64) float64 {
			v := 1 - u
			return 3*v*v*u*p1 + 3*v*u*u*p2 + u*u*u
		}
	}
	xAt := component(x1, x2)
	yAt := component(y1, y2)

	return func(t float64) float64 {
		if t <= 0 {
			return 0
		}
		if t >= 1 {
			return 1
		}
		lo, hi := 0.0, 1.0
		for i := 0; i < 30; i++ {
			mid := (lo + hi) / 2
			if xAt(mid) < t {
				lo = mid
			} else {
				hi = mid
			}
		}
		return yAt((lo + hi) / 2)
	}
}

func clamp01(t float64) float64 {
	switch {
	case t < 0:
		return 0
	case t > 1:
		return 1
	default:
		return t
	}
}

// Named curves, each documented against the CSS string it stands in for
// — every one of these is either spliced verbatim into style.css, or
// (for the hand-rolled AddTickCallback cases) evaluated directly by
// Animate.
var (
	// EmphasizedDecelerate is cubic-bezier(.2, 0, 0, 1): a fast start
	// that settles gently into place. RowInsertDuration and
	// PanelShowDuration/PanelHideDuration all use this curve.
	EmphasizedDecelerate = CubicBezier(.2, 0, 0, 1)

	// Decelerate is cubic-bezier(.4, 0, 1, 1): RowRemoveDuration's own
	// curve — a slower, more linear-feeling exit than
	// EmphasizedDecelerate's entrance.
	Decelerate = CubicBezier(.4, 0, 1, 1)

	// EaseOut is CSS's own `ease-out` keyword, cubic-bezier(0, 0, .58, 1).
	// The default for anything the spec table marks plain "ease-out":
	// hover, focus ring and selection fill (style.css) and checkbox fill
	// (internal/ui/widget's own independent fillEase). NOT what
	// internal/ui/composer's height growth or internal/ui/widget's
	// strikethrough currently use — those are hand-rolled with their own
	// different curve shapes (a cubic polynomial and a linear ramp,
	// respectively), predating this package and not retouched here to
	// avoid changing already-shipped motion for a cosmetic curve match.
	EaseOut = CubicBezier(0, 0, .58, 1)

	// EaseIn is CSS's own `ease-in` keyword, cubic-bezier(.42, 0, 1, 1) —
	// the originally measured curve for ToastOutDuration. internal/ui/
	// toast's shipped CSS actually uses plain `ease` (Ease, below)
	// instead; see ToastOutDuration's own doc comment for why that was
	// left as-is.
	EaseIn = CubicBezier(.42, 0, 1, 1)

	// EaseInOut is CSS's own `ease-in-out` keyword,
	// cubic-bezier(.42, 0, .58, 1). ScrollToInsertedDuration's own curve.
	EaseInOut = CubicBezier(.42, 0, .58, 1)

	// Ease is CSS's own plain `ease` keyword, cubic-bezier(.25, .1, .25, 1).
	// The overlay scrollbar's own curve.
	Ease = CubicBezier(.25, .1, .25, 1)
)
