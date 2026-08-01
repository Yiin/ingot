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

// TestUnfocusedRuleTouchesOnlyTheRing guards copper-l2z.26's
// unfocused-panel contract at the stylesheet level: ".ingot-panel.unfocused"
// must declare only the focus-ring colour override — "leave every other
// colour alone" from the child spec, enforced here so a future edit can't
// quietly widen the rule to dim card fills or text. It used to halve the
// panel's drop shadow too; the panel has no shadow of its own now that it
// is an ordinary toplevel and the compositor draws its frame.
func TestUnfocusedRuleTouchesOnlyTheRing(t *testing.T) {
	re := regexp.MustCompile(`(?s)\.ingot-panel\.unfocused\s*\{([^}]*)\}`)
	m := re.FindStringSubmatch(theme.CSS)
	if m == nil {
		t.Fatalf("style.css has no `.ingot-panel.unfocused { ... }` rule")
	}
	propRE := regexp.MustCompile(`([\w-]+)\s*:`)
	allowed := map[string]bool{"--focus-ring-color": true}
	for _, prop := range propRE.FindAllStringSubmatch(m[1], -1) {
		if !allowed[prop[1]] {
			t.Errorf(".ingot-panel.unfocused declares %q, want only --focus-ring-color", prop[1])
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

// TestFocusVisibleSelectorsAlsoRequireFocus guards the one-widget meaning
// of every focus-ring selector. GTK sets GTK_STATE_FLAG_FOCUS_VISIBLE on
// the focused widget *and every ancestor* (measured live: focusing the
// composer's text view set it on the window, the panel box, the composer
// box and the scrolled window too), so a bare ":focus-visible" selector
// rings the whole ancestor chain. On the layer-shell panel that showed up
// as blue arcs at the four rounded corners plus a second ring around the
// composer. GTK_STATE_FLAG_FOCUSED does not propagate, so pairing the two
// is what keeps a ring on one widget.
func TestFocusVisibleSelectorsAlsoRequireFocus(t *testing.T) {
	stripped := cssCommentRE.ReplaceAllString(theme.CSS, "")
	// Every selector fragment ending in :focus-visible, with whatever
	// pseudo-classes precede it, so ":focus:focus-visible" is
	// distinguishable from a bare ":focus-visible".
	re := regexp.MustCompile(`[^\s,{}]*:focus-visible`)
	found := re.FindAllString(stripped, -1)
	if len(found) == 0 {
		t.Fatal("style.css declares no :focus-visible selector at all")
	}
	for _, sel := range found {
		if !strings.Contains(sel, ":focus:focus-visible") {
			t.Errorf("selector %q uses :focus-visible without :focus, so it rings every ancestor of the focused widget", sel)
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
