package motion

import "testing"

func TestShouldScroll(t *testing.T) {
	cases := []struct {
		name                       string
		pos, firstVisible, lastVis int
		want                       bool
	}{
		{"well above the viewport", 0, 10, 20, true},
		{"well below the viewport", 30, 10, 20, true},
		{"exactly at the top edge, still visible", 10, 10, 20, false},
		{"exactly at the bottom edge, still visible", 20, 10, 20, false},
		{"mid-viewport", 15, 10, 20, false},
		{"one above the top edge", 9, 10, 20, true},
		{"one below the bottom edge", 21, 10, 20, true},
		{"single-row viewport, matches", 5, 5, 5, false},
		{"single-row viewport, misses", 6, 5, 5, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ShouldScroll(c.pos, c.firstVisible, c.lastVis); got != c.want {
				t.Errorf("ShouldScroll(%d, %d, %d) = %v, want %v", c.pos, c.firstVisible, c.lastVis, got, c.want)
			}
		})
	}
}
