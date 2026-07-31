package theme

import (
	"reflect"
	"regexp"
	"sort"
	"testing"
)

// These tests live in package theme rather than theme_test because
// Palette.tokens is unexported: it is an implementation detail of the
// dark provider, not part of the package's surface.

var (
	rootBlockRE = regexp.MustCompile(`(?s):root\s*\{(.*?)\n\}`)
	declRE      = regexp.MustCompile(`(?m)^\s*(--[\w-]+)\s*:\s*(.*?);\s*$`)
	commentRE   = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

// parseRootBlock returns every custom property declared in css's :root
// block, comments stripped the same way css_test.go strips them.
func parseRootBlock(t *testing.T, css string) map[string]string {
	t.Helper()
	m := rootBlockRE.FindStringSubmatch(commentRE.ReplaceAllString(css, ""))
	if m == nil {
		t.Fatalf("no `:root { ... }` block found")
	}
	out := map[string]string{}
	for _, decl := range declRE.FindAllStringSubmatch(m[1], -1) {
		out[decl[1]] = decl[2]
	}
	if len(out) == 0 {
		t.Fatalf(":root block parsed to zero declarations")
	}
	return out
}

// TestLightTokensMatchStylesheet is the golden test tying Palette to
// style.css: every colour the Light palette names must be declared in
// :root under the same custom-property name and with the same value. The
// dark provider overrides exactly these names, so a rename on either side
// would silently leave one token stuck in the light palette.
func TestLightTokensMatchStylesheet(t *testing.T) {
	root := parseRootBlock(t, CSS)
	for name, want := range Light.tokens() {
		got, ok := root[name]
		if !ok {
			t.Errorf("style.css's :root does not declare %s", name)
			continue
		}
		if got != want {
			t.Errorf("style.css's %s = %q, want %q (theme.Light)", name, got, want)
		}
	}
}

// colourValueRE recognises a :root value that names a colour, so the
// reverse golden check below can tell a colour token apart from a size or
// a duration. It is a sound classifier here rather than a lucky
// heuristic: every colour in this stylesheet is a hex literal or a
// functional colour notation, and no non-colour token's value contains
// either. It currently splits the file into 29 colour-valued entries (all
// in the palette) and 44 non-colour ones — sizes, durations,
// --font-family, --toast-light-blur, --tracking-section, and
// --focus-ring-color, whose value is var(--accent) and so correctly does
// not match. A composite value that merely contains a colour, like
// --panel-shadow, does match, which is what we want: those flip too.
//
// The alternatives are covered so a token written in a form this file
// does not use yet cannot slip past the check: hsl/hwb/lab/lch/oklab/
// oklch, colour keywords, color() and color-mix(). Known limits, both
// acceptable because they mean a false POSITIVE (a spurious failure
// demanding a look) rather than a silent miss: a keyword is matched by
// word only, so a bare `currentcolor` or an unusual keyword like
// `rebeccapurple` is not recognised, and a non-colour value that happens
// to contain the word "white" would be misread as a colour.
var colourValueRE = regexp.MustCompile(
	`#[0-9a-fA-F]{3,8}` +
		`|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color|color-mix)\s*\(` +
		`|\b(?:transparent|white|black|gray|grey|red|green|blue|currentColor)\b`)

// TestStylesheetColourTokensAreInThePalette is the reverse of
// TestLightTokensMatchStylesheet, and it guards this bug's exact class: a
// colour custom property declared in :root but absent from Palette passes
// every other test in this file and then stays light forever, because the
// dark provider is generated from the palette and would never override
// it.
func TestStylesheetColourTokensAreInThePalette(t *testing.T) {
	light := Light.tokens()
	for name, value := range parseRootBlock(t, CSS) {
		if !colourValueRE.MatchString(value) {
			continue
		}
		if _, ok := light[name]; !ok {
			t.Errorf("style.css declares the colour %s: %s, but theme.Palette has no field for it — it would never switch to dark", name, value)
		}
	}
}

// TestPaletteFieldsAreTokensOrExempt keeps tokens() exhaustive over
// Palette. Without it, a new colour field could be added to the struct
// and to both palettes yet never reach the generated dark sheet, which
// looks correct in every other test here.
func TestPaletteFieldsAreTokensOrExempt(t *testing.T) {
	// HighlightBg is the one deliberate exemption: it is a Pango markup
	// attribute (internal/ui/mdpango), never a CSS custom property, so it
	// has no :root entry to override. Anything else must be a token.
	exempt := map[string]bool{"HighlightBg": true}

	typ := reflect.TypeOf(Palette{})
	fields := map[string]bool{}
	for i := range typ.NumField() {
		fields[typ.Field(i).Name] = true
	}

	// Check the exemption still names something real. Renaming the field
	// would otherwise leave a stale entry here that silently keeps
	// excusing a field that no longer exists.
	for name := range exempt {
		if !fields[name] {
			t.Errorf("the exemption list names %q, which is not a Palette field — rename or drop it", name)
		}
	}

	// Compare by name set, not by count: two token names mapping onto the
	// same field would keep the counts balanced while leaving another
	// field with no CSS property of its own.
	byField := tokenNamesByField(t)
	for field := range fields {
		names := byField[field]
		switch {
		case exempt[field] && len(names) > 0:
			t.Errorf("Palette.%s is listed exempt but tokens() emits %v for it", field, names)
		case exempt[field]:
		case len(names) == 0:
			t.Errorf("Palette.%s has no entry in tokens(), so the dark provider would never override it", field)
		case len(names) > 1:
			t.Errorf("tokens() maps %v all onto Palette.%s, so some other field is going without", names, field)
		}
	}

	// A field left empty in either palette was added to the struct but
	// never wired into the palette literals below it.
	light, dark := reflect.ValueOf(Light), reflect.ValueOf(Dark)
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		if light.Field(i).String() == "" {
			t.Errorf("theme.Light leaves %s empty", name)
		}
		if dark.Field(i).String() == "" {
			t.Errorf("theme.Dark leaves %s empty", name)
		}
	}
}

// tokenNamesByField reports, for each Palette field, every CSS property
// tokens() emits from it.
//
// It works by calling tokens() on a probe Palette whose every field holds
// its own field name as its value, so each emitted value IS the name of
// the field it came from. Matching on a real palette's values instead
// would not work: Light deliberately reuses literals (ToastLightText is
// defined as Ink, and #FFFFFF is CardBgHover, ToastDarkText,
// ToastIconTick and MenuBg all at once), so a value is not an identity.
func tokenNamesByField(t *testing.T) map[string][]string {
	t.Helper()
	typ := reflect.TypeOf(Palette{})
	probe := reflect.New(typ).Elem()
	for i := range typ.NumField() {
		probe.Field(i).SetString(typ.Field(i).Name)
	}

	byField := map[string][]string{}
	for name, field := range probe.Interface().(Palette).tokens() {
		byField[field] = append(byField[field], name)
	}
	for field, names := range byField {
		sort.Strings(names)
		byField[field] = names
	}
	return byField
}

// TestDarkTokensCoverLightTokens fails when a colour token gains a light
// value but no dark one (or the other way round), which would otherwise
// show up only as one widget stuck in the wrong scheme.
func TestDarkTokensCoverLightTokens(t *testing.T) {
	light, dark := Light.tokens(), Dark.tokens()
	for name := range light {
		if _, ok := dark[name]; !ok {
			t.Errorf("theme.Dark has no value for %s", name)
		}
	}
	for name := range dark {
		if _, ok := light[name]; !ok {
			t.Errorf("theme.Light has no value for %s", name)
		}
	}
}

// TestDarkOverrideCSSIsDeterministic checks the generated dark sheet
// parses back to exactly the dark token set, and that two calls produce
// byte-equal output — the sorted emission order is what makes the
// generated sheet diffable.
func TestDarkOverrideCSSIsDeterministic(t *testing.T) {
	first, second := darkOverrideCSS(), darkOverrideCSS()
	if first != second {
		t.Fatalf("darkOverrideCSS() is not deterministic:\n%s\nvs\n%s", first, second)
	}

	got := parseRootBlock(t, first)
	want := Dark.tokens()
	if len(got) != len(want) {
		t.Errorf("generated :root has %d declarations, want %d", len(got), len(want))
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("generated %s = %q, want %q", name, got[name], value)
		}
	}
}
