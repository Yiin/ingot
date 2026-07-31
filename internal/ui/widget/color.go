package widget

import "fmt"

// hexRGB parses a "#RRGGBB" literal — the form every colour token in
// internal/ui/theme uses — into cairo's 0..1 float components. It panics
// on a malformed literal: every caller passes a theme constant, never
// user input.
func hexRGB(hex string) (r, g, b float64) {
	var ri, gi, bi int
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &ri, &gi, &bi); err != nil {
		panic("widget: malformed hex colour " + hex)
	}
	return float64(ri) / 255, float64(gi) / 255, float64(bi) / 255
}
