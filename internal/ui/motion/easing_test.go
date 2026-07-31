package motion

import "testing"

func TestCubicBezierEndpoints(t *testing.T) {
	f := CubicBezier(0.2, 0, 0, 1)
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

func TestCubicBezierMonotonicForNamedCurves(t *testing.T) {
	curves := map[string]Easing{
		"EmphasizedDecelerate": EmphasizedDecelerate,
		"Decelerate":           Decelerate,
		"EaseOut":              EaseOut,
		"EaseIn":               EaseIn,
		"EaseInOut":            EaseInOut,
		"Ease":                 Ease,
	}
	steps := []float64{0, 0.1, 0.25, 0.4, 0.5, 0.6, 0.75, 0.9, 1}
	for name, ease := range curves {
		prev := -1.0
		for _, t2 := range steps {
			v := ease(t2)
			if v < 0 || v > 1 {
				t.Errorf("%s(%v) = %v, want within [0,1]", name, t2, v)
			}
			if v < prev {
				t.Errorf("%s is not monotonic: value decreased at t=%v (%v -> %v)", name, t2, prev, v)
			}
			prev = v
		}
		if got := ease(0); got != 0 {
			t.Errorf("%s(0) = %v, want 0", name, got)
		}
		if got := ease(1); got != 1 {
			t.Errorf("%s(1) = %v, want 1", name, got)
		}
	}
}

func TestClamp01(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{-0.5, 0},
		{0, 0},
		{0.5, 0.5},
		{1, 1},
		{1.5, 1},
	}
	for _, c := range cases {
		if got := clamp01(c.in); got != c.want {
			t.Errorf("clamp01(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
