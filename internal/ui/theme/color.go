package theme

import (
	"fmt"
	"strconv"
	"strings"
)

// The two colour literal forms every token in this package uses, parsed
// into cairo's 0..1 float components. They live here rather than in each
// drawing package because internal/ui/widget, internal/ui/notelist and
// internal/ui/toast all paint theme colours by hand and would otherwise
// carry three copies of the same twenty lines.

// ParseRGB parses a "#RRGGBB" literal. It panics on a malformed literal:
// every caller passes a theme colour, never user input.
func ParseRGB(hex string) (r, g, b float64) {
	// The length check is not redundant with Sscanf. Sscanf stops once its
	// verbs are satisfied and never looks at what follows, so "#FFFFFF00"
	// would silently parse as "#FFFFFF" — dropping an alpha channel the
	// caller clearly meant. HighlightBg is exactly that 8-digit form, so
	// handing it to this function must fail loudly, not quietly.
	var ri, gi, bi int
	if len(hex) != 7 {
		panic("theme: malformed hex colour " + hex)
	}
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &ri, &gi, &bi); err != nil {
		panic("theme: malformed hex colour " + hex)
	}
	return float64(ri) / 255, float64(gi) / 255, float64(bi) / 255
}

// ParseRGBA parses an "rgba(r,g,b,a)" literal — the form Rule,
// PlaceholderBorder and ScrollbarInk use. It panics on a malformed
// literal, for the same reason ParseRGB does.
func ParseRGBA(s string) (r, g, b, a float64) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "rgba(") || !strings.HasSuffix(s, ")") {
		panic("theme: malformed rgba() colour " + s)
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(s, "rgba("), ")")
	parts := strings.Split(inner, ",")
	if len(parts) != 4 {
		panic("theme: malformed rgba() colour " + s)
	}

	channel := func(i int) float64 {
		v, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
		if err != nil {
			panic(fmt.Sprintf("theme: malformed rgba() colour %s: %v", s, err))
		}
		return v
	}

	return channel(0) / 255, channel(1) / 255, channel(2) / 255, channel(3)
}
