package widget

import "time"

// cubicBezier returns the easing function for the CSS
// cubic-bezier(x1, y1, x2, y2) timing curve anchored at (0,0) and (1,1) —
// the same curve family style.css uses for its own transitions. It solves
// for the curve parameter u at x(u) == t by bisection, then evaluates y(u).
func cubicBezier(x1, y1, x2, y2 float64) func(t float64) float64 {
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

// The checkbox's 220ms animation, per the child spec: fill eases out over
// the first 120ms, then the tick sweeps over its own 140ms starting 40ms
// before the fill finishes (an 80ms mark), for a 220ms total.
const (
	fillDuration  = 120 * time.Millisecond
	tickDuration  = 140 * time.Millisecond
	tickOverlap   = 40 * time.Millisecond
	tickStart     = fillDuration - tickOverlap
	checkDuration = fillDuration + tickDuration - tickOverlap
)

var (
	// fillEase is CSS's own ease-out.
	fillEase = cubicBezier(0, 0, 0.58, 1)
	// tickEase is the spec's own curve for the tick stroke sweep.
	tickEase = cubicBezier(0.2, 0, 0, 1)
)

// checkProgress returns how far the fill and the tick sweep have reached
// at elapsed into the checkbox's checkDuration animation.
func checkProgress(elapsed time.Duration) (fill, tick float64) {
	fill = fillEase(clamp01(float64(elapsed) / float64(fillDuration)))
	tick = tickEase(clamp01(float64(elapsed-tickStart) / float64(tickDuration)))
	return fill, tick
}
