package widget

import (
	"testing"
	"time"
)

func TestCubicBezierEndpoints(t *testing.T) {
	f := cubicBezier(0.2, 0, 0, 1)
	if got := f(0); got != 0 {
		t.Errorf("f(0) = %v, want 0", got)
	}
	if got := f(1); got != 1 {
		t.Errorf("f(1) = %v, want 1", got)
	}
	if got := f(-1); got != 0 {
		t.Errorf("f(-1) = %v, want 0 (clamped)", got)
	}
	if got := f(2); got != 1 {
		t.Errorf("f(2) = %v, want 1 (clamped)", got)
	}
}

func TestCheckProgressBoundaries(t *testing.T) {
	if fill, tick := checkProgress(0); fill != 0 || tick != 0 {
		t.Errorf("checkProgress(0) = (%v, %v), want (0, 0)", fill, tick)
	}
	if fill, tick := checkProgress(checkDuration); fill != 1 || tick != 1 {
		t.Errorf("checkProgress(checkDuration) = (%v, %v), want (1, 1)", fill, tick)
	}
}

// TestCheckProgressSnapshotsAreDistinct encodes the acceptance criteria
// directly: snapshots at t=0/110/220ms must be visually distinct.
func TestCheckProgressSnapshotsAreDistinct(t *testing.T) {
	fill0, tick0 := checkProgress(0)
	fill110, tick110 := checkProgress(110 * time.Millisecond)
	fill220, tick220 := checkProgress(220 * time.Millisecond)

	if fill0 == fill110 && tick0 == tick110 {
		t.Error("t=0ms and t=110ms snapshots are identical")
	}
	if fill110 == fill220 && tick110 == tick220 {
		t.Error("t=110ms and t=220ms snapshots are identical")
	}
	if fill0 == fill220 && tick0 == tick220 {
		t.Error("t=0ms and t=220ms snapshots are identical")
	}
}

func TestCheckProgressMonotonic(t *testing.T) {
	elapsedsMs := []int64{0, 20, 40, 80, 110, 150, 180, 220}
	var prevFill, prevTick float64
	for i, ms := range elapsedsMs {
		fill, tick := checkProgress(time.Duration(ms) * time.Millisecond)
		if fill < 0 || fill > 1 {
			t.Fatalf("checkProgress(%dms) fill = %v, want within [0,1]", ms, fill)
		}
		if tick < 0 || tick > 1 {
			t.Fatalf("checkProgress(%dms) tick = %v, want within [0,1]", ms, tick)
		}
		if i > 0 {
			if fill < prevFill {
				t.Errorf("fill decreased between %dms and %dms: %v -> %v", elapsedsMs[i-1], ms, prevFill, fill)
			}
			if tick < prevTick {
				t.Errorf("tick decreased between %dms and %dms: %v -> %v", elapsedsMs[i-1], ms, prevTick, tick)
			}
		}
		prevFill, prevTick = fill, tick
	}
}
