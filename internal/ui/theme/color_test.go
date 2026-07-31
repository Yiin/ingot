package theme_test

import (
	"testing"

	"github.com/Yiin/ingot/internal/ui/theme"
)

func TestParseRGB(t *testing.T) {
	cases := []struct {
		in      string
		r, g, b float64
	}{
		{"#000000", 0, 0, 0},
		{"#FFFFFF", 1, 1, 1},
		{theme.Accent, 10.0 / 255, 108.0 / 255, 255.0 / 255},
		{theme.DarkAccent, 76.0 / 255, 141.0 / 255, 255.0 / 255},
	}
	for _, c := range cases {
		r, g, b := theme.ParseRGB(c.in)
		if r != c.r || g != c.g || b != c.b {
			t.Errorf("ParseRGB(%q) = (%v,%v,%v), want (%v,%v,%v)", c.in, r, g, b, c.r, c.g, c.b)
		}
	}
}

func TestParseRGBPanicsOnMalformedInput(t *testing.T) {
	// "#FFFFFF00" is the case worth naming: Sscanf alone stops after three
	// verbs and would parse it as "#FFFFFF", silently discarding the alpha.
	// theme.HighlightBg is that 8-digit form, so this must panic rather
	// than return a plausible-looking wrong colour.
	for _, in := range []string{"rgba(0,0,0,.5)", "#FFFFFF00", "#FFF", "", "FFFFFF"} {
		t.Run(in, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("ParseRGB(%q) did not panic", in)
				}
			}()
			theme.ParseRGB(in)
		})
	}
}

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
		r, g, b, a := theme.ParseRGBA(c.in)
		if r != c.r || g != c.g || b != c.b || a != c.a {
			t.Errorf("ParseRGBA(%q) = (%v,%v,%v,%v), want (%v,%v,%v,%v)", c.in, r, g, b, a, c.r, c.g, c.b, c.a)
		}
	}
}

func TestParseRGBAPanicsOnMalformedInput(t *testing.T) {
	// The trailing-garbage cases ("...))", "... x") are covered by
	// strconv.ParseFloat rejecting the last channel outright, but they are
	// pinned here so the guarantee survives a rewrite of the channel loop.
	for _, in := range []string{"#FF0000", "rgba(0,0,0)", "rgba(0,0,0,1))", "rgba(0,0,0,1 x)", "rgb(0,0,0)"} {
		t.Run(in, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("ParseRGBA(%q) did not panic", in)
				}
			}()
			theme.ParseRGBA(in)
		})
	}
}
