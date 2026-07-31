package notelist

import (
	"testing"

	"github.com/Yiin/ingot/internal/ui/theme"
)

func TestParseRGBA(t *testing.T) {
	cases := []struct {
		in         string
		r, g, b, a float64
	}{
		{theme.Rule, 0, 0, 0, 0.08},
		{theme.PlaceholderBorder, 0, 0, 0, 0.12},
		{theme.ScrollbarInk, 0, 0, 0, 0.28},
		{"rgba(255,255,255,1)", 1, 1, 1, 1},
	}
	for _, c := range cases {
		r, g, b, a := parseRGBA(c.in)
		if r != c.r || g != c.g || b != c.b || a != c.a {
			t.Errorf("parseRGBA(%q) = (%v,%v,%v,%v), want (%v,%v,%v,%v)", c.in, r, g, b, a, c.r, c.g, c.b, c.a)
		}
	}
}

func TestParseRGBAPanicsOnMalformedInput(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Errorf("parseRGBA did not panic on a malformed literal")
		}
	}()
	parseRGBA("#FF0000")
}
