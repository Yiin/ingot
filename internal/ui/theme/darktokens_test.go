package theme_test

import (
	"testing"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// TestDarkTokensMatchSpec is TestTokensMatchSpec's counterpart for the
// dark palette. Without it nothing pins a dark value at all: every other
// test in this package checks the SHAPE of the palettes (same key set,
// every field non-empty, every CSS colour accounted for), so a typo in a
// hex literal changes what ships and fails nothing.
//
// Unlike the light values these are not measurements — no demo footage of
// a dark variant exists — so this is a contract test, not a spec test.
// The values encode the relationships tokens.go's Dark block documents
// (panel darkest, card warm, field cool, selected blue-tinted). Changing
// one deliberately means changing it here too; the point is that it
// cannot happen by accident.
func TestDarkTokensMatchSpec(t *testing.T) {
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"DarkPanelBg", theme.DarkPanelBg, "#1B1C1F"},
		{"DarkPanelRim", theme.DarkPanelRim, "rgba(255,255,255,.12)"},
		{"DarkPanelShadow", theme.DarkPanelShadow, "0 8px 28px rgba(0,0,0,.55), 0 2px 8px rgba(0,0,0,.35)"},
		{"DarkPanelShadowUnfocused", theme.DarkPanelShadowUnfocused, "0 4px 14px rgba(0,0,0,.35), 0 1px 4px rgba(0,0,0,.18)"},

		{"DarkCardBg", theme.DarkCardBg, "#282725"},
		{"DarkCardBgHover", theme.DarkCardBgHover, "#333230"},
		{"DarkCardBgSelected", theme.DarkCardBgSelected, "#1E3555"},
		{"DarkFieldBg", theme.DarkFieldBg, "#25272B"},

		{"DarkInk", theme.DarkInk, "#E9EAEC"},
		{"DarkInkMuted", theme.DarkInkMuted, "#9A9DA3"},
		{"DarkInkDone", theme.DarkInkDone, "#8C8F95"},
		{"DarkRule", theme.DarkRule, "rgba(255,255,255,.10)"},

		{"DarkCheckRing", theme.DarkCheckRing, "#8B8E94"},
		{"DarkAccent", theme.DarkAccent, "#4C8DFF"},
		{"DarkFocusRingDim", theme.DarkFocusRingDim, "rgba(76,141,255,.45)"},

		{"DarkPlaceholderBorder", theme.DarkPlaceholderBorder, "rgba(255,255,255,.14)"},
		{"DarkScrollbarInk", theme.DarkScrollbarInk, "rgba(255,255,255,.30)"},

		{"DarkToastDarkBg", theme.DarkToastDarkBg, "#232326"},
		{"DarkToastDarkText", theme.DarkToastDarkText, "#FFFFFF"},
		{"DarkToastLightBg", theme.DarkToastLightBg, "rgba(58,58,62,.72)"},
		{"DarkToastLightText", theme.DarkToastLightText, "#E9EAEC"},

		{"DarkDragShadow", theme.DarkDragShadow, "0 6px 18px rgba(0,0,0,.55)"},
		{"DarkReject", theme.DarkReject, "#FF6369"},
		{"DarkToastIconBg", theme.DarkToastIconBg, "#E9EAEC"},
		{"DarkToastIconTick", theme.DarkToastIconTick, "#1B1C1F"},
		{"DarkSelectionBg", theme.DarkSelectionBg, "rgba(76,141,255,.35)"},
		{"DarkOverlayHover", theme.DarkOverlayHover, "rgba(255,255,255,.08)"},
		{"DarkOverlayActive", theme.DarkOverlayActive, "rgba(255,255,255,.13)"},
		{"DarkMenuBg", theme.DarkMenuBg, "#2E2D31"},
		{"DarkHighlightBg", theme.DarkHighlightBg, "#4C8DFF33"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("theme.%s = %#v, want %#v", tt.name, tt.got, tt.want)
		}
	}
}

// TestNewLightTokensMatchSpec pins the light tokens this change added.
// They are not in TestTokensMatchSpec because that test guards values
// measured from the demo video, and these are not measurements: four are
// literals lifted verbatim out of style.css and internal/ui/toast (so
// their want values ARE the pre-change behaviour, and must not drift),
// and the rest are new names for surfaces the light spec never had to
// name because nothing overrode them.
func TestNewLightTokensMatchSpec(t *testing.T) {
	tests := []struct {
		name string
		got  any
		want any
	}{
		// Lifted from style.css's .note-card.dragging and .composer.reject.
		{"DragShadow", theme.DragShadow, "0 6px 16px rgba(0,0,0,.18)"},
		{"Reject", theme.Reject, "#E5484D"},
		// Lifted from internal/ui/toast/icon.go's cairo SetSourceRGB pair.
		{"ToastIconBg", theme.ToastIconBg, "#000000"},
		{"ToastIconTick", theme.ToastIconTick, "#FFFFFF"},
		// Lifted from internal/ui/mdpango, where it was a package constant.
		{"HighlightBg", theme.HighlightBg, "#0A6CFF1F"},

		{"SelectionBg", theme.SelectionBg, "rgba(10,108,255,.28)"},
		{"OverlayHover", theme.OverlayHover, "rgba(0,0,0,.06)"},
		{"OverlayActive", theme.OverlayActive, "rgba(0,0,0,.10)"},
		{"MenuBg", theme.MenuBg, "#FFFFFF"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("theme.%s = %#v, want %#v", tt.name, tt.got, tt.want)
		}
	}
}
