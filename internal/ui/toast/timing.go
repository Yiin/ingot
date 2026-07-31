package toast

import "time"

// Timing per the epic's frame-by-frame measurement, then adjusted: 140ms
// scale(0.94)->1 in, hold 1200ms, 120ms translateY(0->4px) out. Kept in
// sync with style.css's .toast-in/.toast-out animation durations by
// css_test.go.
const (
	FadeInDuration  = 140 * time.Millisecond
	HoldDuration    = 1200 * time.Millisecond
	FadeOutDuration = 120 * time.Millisecond
)
