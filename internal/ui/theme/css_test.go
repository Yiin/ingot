package theme_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Yiin/ingot/internal/ui/theme"
)

var cssCommentRE = regexp.MustCompile(`(?s)/\*.*?\*/`)

// gtkRejectedProperties are CSS properties GTK 4.22 does not support; each
// logs "No property named ..." at parse time (verified by running code,
// see style.css's header comment). A stylesheet must never declare one.
var gtkRejectedProperties = []string{
	"display", "position", "top", "z-index", "cursor", "will-change",
	"overflow", "aspect-ratio",
}

// TestStylesheetHasNoRejectedProperties fails if style.css declares a
// property GTK 4.22 silently drops with a "No property named ..." log
// line, so a future edit can't quietly reintroduce one.
func TestStylesheetHasNoRejectedProperties(t *testing.T) {
	stripped := cssCommentRE.ReplaceAllString(theme.CSS, "")
	for _, prop := range gtkRejectedProperties {
		re := regexp.MustCompile(`(?m)(^|[;{])\s*` + regexp.QuoteMeta(prop) + `\s*:`)
		if re.MatchString(stripped) {
			t.Errorf("style.css declares the GTK-rejected property %q", prop)
		}
	}
}

// TestUnfocusedRuleTouchesOnlyRingAndShadow guards copper-l2z.26's
// unfocused-panel contract at the stylesheet level: ".ingot-panel.unfocused"
// must declare only the focus-ring colour override and the panel shadow —
// "leave every other colour alone" from the child spec, enforced here so
// a future edit can't quietly widen the rule to dim card fills or text.
func TestUnfocusedRuleTouchesOnlyRingAndShadow(t *testing.T) {
	re := regexp.MustCompile(`(?s)\.ingot-panel\.unfocused\s*\{([^}]*)\}`)
	m := re.FindStringSubmatch(theme.CSS)
	if m == nil {
		t.Fatalf("style.css has no `.ingot-panel.unfocused { ... }` rule")
	}
	propRE := regexp.MustCompile(`([\w-]+)\s*:`)
	allowed := map[string]bool{"--focus-ring-color": true, "box-shadow": true}
	for _, prop := range propRE.FindAllStringSubmatch(m[1], -1) {
		if !allowed[prop[1]] {
			t.Errorf(".ingot-panel.unfocused declares %q, want only --focus-ring-color and box-shadow", prop[1])
		}
	}
}

// TestStylesheetDefinesRequiredClasses checks every class copper-l2z.18's
// spec names is actually present in the embedded stylesheet.
func TestStylesheetDefinesRequiredClasses(t *testing.T) {
	required := []string{
		".ingot-panel", ".note-card", ".note-card:hover", ".note-card.selected",
		".note-card.done", ".note-card.just-inserted", ".section-header",
		".section-rule", ".search-field", ".composer", ".toast-dark",
		".toast-light", ".ingot-notelist", ".note-placeholder", ".ingot-scrollbar",
	}
	for _, class := range required {
		if !strings.Contains(theme.CSS, class) {
			t.Errorf("style.css does not define %q", class)
		}
	}
}

// TestFontFamilyIsWired guards against theme.FontFamily going dead again:
// it was defined but never referenced by any font-family declaration,
// so the bundled Inter Variable font was registered but never actually
// used anywhere (see copper-doi).
func TestFontFamilyIsWired(t *testing.T) {
	if !strings.Contains(theme.CSS, "--font-family: "+theme.FontFamily+";") {
		t.Errorf("style.css's --font-family does not match theme.FontFamily (%s)", theme.FontFamily)
	}
	fontFamilyDeclRE := regexp.MustCompile(`(?m)^\s*font-family:\s*var\(--font-family\);`)
	if n := len(fontFamilyDeclRE.FindAllString(theme.CSS, -1)); n == 0 {
		t.Error("style.css never declares `font-family: var(--font-family)` anywhere")
	}
}
