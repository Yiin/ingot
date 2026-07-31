package theme_test

import (
	"testing"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// TestTokensMatchSpec is the golden test for copper-l2z.18's design
// tokens: every exported constant must equal the value measured from the
// original app's demo video (see tokens.go's package doc). If this test
// fails, the constant is wrong — do not edit the want value to match it.
func TestTokensMatchSpec(t *testing.T) {
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"PanelWidth", theme.PanelWidth, 360},
		{"PanelHeight", theme.PanelHeight, 640},
		{"PanelMarginEdge", theme.PanelMarginEdge, 12},
		{"PanelRadius", theme.PanelRadius, 32},
		{"PanelBg", theme.PanelBg, "#E5E6E9"},
		{"PanelRim", theme.PanelRim, "rgba(255,255,255,.40)"},
		{"ContentInset", theme.ContentInset, 15},
		{"PanelShadow", theme.PanelShadow, "0 8px 28px rgba(0,0,0,.20), 0 2px 8px rgba(0,0,0,.08)"},
		{"PanelPadTop", theme.PanelPadTop, 16},
		{"PanelPadBottom", theme.PanelPadBottom, 15},

		{"CardBg", theme.CardBg, "#F8F8F6"},
		{"CardBgHover", theme.CardBgHover, "#FFFFFF"},
		{"CardBgSelected", theme.CardBgSelected, "#EFF6FF"},
		{"CardRadius", theme.CardRadius, 16},
		{"CardPadY", theme.CardPadY, 8},
		{"CardPadL", theme.CardPadL, 12},
		{"CardPadR", theme.CardPadR, 14},
		{"CardGap", theme.CardGap, 12},
		{"CardMinHeight", theme.CardMinHeight, 34},

		{"FieldBg", theme.FieldBg, "#F3F4F7"},
		{"SearchHeight", theme.SearchHeight, 30},
		{"SearchRadius", theme.SearchRadius, 15},
		{"OverflowButton", theme.OverflowButton, 30},
		{"SearchGap", theme.SearchGap, 8},
		{"ComposerMinHeight", theme.ComposerMinHeight, 58},

		{"FontBody", theme.FontBody, 14},
		{"LineBody", theme.LineBody, 18},
		{"FontSection", theme.FontSection, 11},
		{"TrackingSection", theme.TrackingSection, "0.07em"},
		{"FontToast", theme.FontToast, 14},

		{"Ink", theme.Ink, "#1C1C1E"},
		{"InkMuted", theme.InkMuted, "#6E7073"},
		{"InkDone", theme.InkDone, "#6E6E73"},
		{"Rule", theme.Rule, "rgba(0,0,0,.08)"},

		{"CheckSize", theme.CheckSize, 17},
		{"CheckStroke", theme.CheckStroke, 1.5},
		{"CheckRing", theme.CheckRing, "#7A7A7C"},
		{"Accent", theme.Accent, "#0A6CFF"},
		{"FocusRing", theme.FocusRing, 2},

		{"SectionMarginTop", theme.SectionMarginTop, 21},
		{"SectionMarginBottom", theme.SectionMarginBottom, 21},
		{"SectionInset", theme.SectionInset, 11},
		{"SectionRuleGap", theme.SectionRuleGap, 10},

		{"ToastHeight", theme.ToastHeight, 34},
		{"ToastRadius", theme.ToastRadius, 17},
		{"ToastPadX", theme.ToastPadX, 16},

		{"PlaceholderBorder", theme.PlaceholderBorder, "rgba(0,0,0,.12)"},
		{"ScrollbarWidth", theme.ScrollbarWidth, 5},
		{"ScrollbarInset", theme.ScrollbarInset, 3},
		{"ScrollbarInk", theme.ScrollbarInk, "rgba(0,0,0,.28)"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("theme.%s = %#v, want %#v", tt.name, tt.got, tt.want)
		}
	}
}
