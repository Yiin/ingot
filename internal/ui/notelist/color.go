package notelist

import (
	"fmt"
	"strconv"
	"strings"
)

// parseRGBA parses the "rgba(r,g,b,a)" literals theme's Rule,
// PlaceholderBorder and ScrollbarInk tokens use into cairo's 0..1 float
// components. It panics on a malformed literal: every caller passes a
// theme constant, never user input.
func parseRGBA(s string) (r, g, b, a float64) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "rgba(") || !strings.HasSuffix(s, ")") {
		panic("notelist: malformed rgba() colour " + s)
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(s, "rgba("), ")")
	parts := strings.Split(inner, ",")
	if len(parts) != 4 {
		panic("notelist: malformed rgba() colour " + s)
	}

	channel := func(i int) float64 {
		v, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
		if err != nil {
			panic(fmt.Sprintf("notelist: malformed rgba() colour %s: %v", s, err))
		}
		return v
	}

	return channel(0) / 255, channel(1) / 255, channel(2) / 255, channel(3)
}
