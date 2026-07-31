// Package theme is the single source of every colour, size and duration in
// the Ingot panel: the design tokens below, the embedded stylesheet
// (style.css) built from them, and the bundled Inter Variable font.
//
// Every token was measured frame by frame from the original app's demo
// video and calibrated against the macOS traffic-light button pitch —
// do not "improve" them. dp means GTK logical px, already scaled 1.15x
// from the measured macOS points (which lands body text on 14 px).
package theme

// Panel: the top-level opaque card the whole app lives in. It is
// deliberately opaque (#E5E6E9) with no backdrop blur — measured at demo
// frame 12s against a wallpaper ranging from rgb(8,125,203) to
// rgb(185,206,209), the panel reads exactly rgb(229,230,233).
const (
	PanelWidth      = 360
	PanelHeight     = 640
	PanelMarginEdge = 12
	PanelRadius     = 32
	PanelBg         = "#E5E6E9"
	PanelRim        = "rgba(255,255,255,.40)"
	ContentInset    = 15
	PanelShadow     = "0 8px 28px rgba(0,0,0,.20), 0 2px 8px rgba(0,0,0,.08)"
	PanelPadTop     = 16
	PanelPadBottom  = 15
)

// Note card: warm surface, deliberately distinct from the cool
// search/composer surface below. Cards have no drop shadow — the panel
// background sits flat 1px below a card at demo frame 17s; the shadows
// visible in later shots are the video's colour grade.
const (
	CardBg         = "#F8F8F6"
	CardBgHover    = "#FFFFFF"
	CardBgSelected = "#EFF6FF"
	CardRadius     = 16
	CardPadY       = 8
	CardPadL       = 12
	CardPadR       = 14
	CardGap        = 12
	CardMinHeight  = 34
)

// Search field and composer: cool surface, deliberately distinct from the
// warm note card above.
const (
	FieldBg           = "#F3F4F7"
	SearchHeight      = 30
	SearchRadius      = 15
	OverflowButton    = 30
	SearchGap         = 8
	ComposerMinHeight = 58
)

// Type.
const (
	FontBody        = 14
	LineBody        = 18
	FontSection     = 11
	TrackingSection = "0.07em"
	FontToast       = 14
)

// Ink: text colours.
const (
	Ink      = "#1C1C1E"
	InkMuted = "#6E7073"
	InkDone  = "#6E6E73"
	Rule     = "rgba(0,0,0,.08)"
)

// Checkbox and focus ring.
const (
	CheckSize   = 17
	CheckStroke = 1.5
	CheckRing   = "#7A7A7C"
	Accent      = "#0A6CFF"
	FocusRing   = 2
)

// Section headers.
const (
	SectionMarginTop    = 21
	SectionMarginBottom = 21
	SectionInset        = 11
	SectionRuleGap      = 10
)

// Toast: the dark global HUD and the light in-panel toast. These are two
// different widgets, not one widget in two colours — measured frame by
// frame from the demo (11.0s/13.77s for the dark HUD's position, 40.5s
// for the light toast's fill, position, and icon). See internal/ui/toast.
const (
	ToastHeight = 34
	ToastRadius = 17
	ToastPadX   = 16

	// ToastDarkBg is opaque near-black, measured directly off the HUD
	// fill — deliberately distinct from --ink (#1C1C1E), which is the
	// panel's body text colour, not this.
	ToastDarkBg   = "#0B0B0B"
	ToastDarkText = "#FFFFFF"

	// ToastLightBg is translucent, not the opaque CardBgHover a naive
	// reading of "light toast" would reach for: frame 40.5s shows the
	// focus ring and text of the card behind it showing through, so this
	// is a vibrancy material — paired with ToastLightBlur's
	// backdrop-filter, not a flat fill.
	ToastLightBg   = "rgba(255,255,255,.72)"
	ToastLightBlur = "blur(20px)"
	ToastLightText = Ink

	// ToastIconSize/ToastIconGap describe the light toast's filled black
	// circle with a white tick: ~14dp, 8dp before the text. The dark HUD
	// never has an icon.
	ToastIconSize = 14
	ToastIconGap  = 8

	// HUDMarginBottom is how far the dark HUD's layer-shell surface
	// bottom sits above the output bottom — measured centre x = 961 on a
	// 1920-wide frame at both 11.0s and 13.77s (i.e. screen-centred
	// horizontally, achieved by anchoring neither left nor right edge),
	// ~195dp above the bottom edge.
	HUDMarginBottom = 195

	// PanelToastGap is the light toast's default clearance above the
	// panel's bottom edge — measured centre x 1325 vs panel centre 1324
	// at frame 40.5s, ~20dp above the composer. This is only the
	// fallback used before the composer exists: the panel assembler
	// (copper-l2z.26) should call InPanel.SetBottomInset with the
	// composer's live height plus this gap once that widget exists, so
	// the toast tracks the composer as it grows.
	PanelToastGap = 20
)

// Empty-section placeholder card and the notelist's overlay scrollbar
// (internal/ui/notelist). ScrollbarInk is a contract value, not a
// measurement — like .dragging, the original has no comparable affordance
// to measure against.
const (
	PlaceholderBorder = "rgba(0,0,0,.12)"
	ScrollbarWidth    = 5
	ScrollbarInset    = 3
	ScrollbarInk      = "rgba(0,0,0,.28)"
)

// Unfocused panel state (internal/ui/panel): never shown in the demo, so
// this is Ingot's own contract, not a measurement. Only the focus-ring
// family (*:focus-visible, .note-card.selected/.selection-anchor,
// .composer.focused) dims to 45% opacity, and the panel shadow halves —
// every other colour (fills, text, done state) stays exactly as-is.
const (
	FocusRingDim         = "rgba(10,108,255,.45)"
	PanelShadowUnfocused = "0 4px 14px rgba(0,0,0,.10), 0 1px 4px rgba(0,0,0,.04)"
)

// Duplicate-capture flash (internal/ui/panel + internal/ui/notelist):
// the existing row's ring pulses twice over 300ms when a capture
// duplicates the newest note. Not shown in the original — Ingot's own
// contract, like .dragging.
const DuplicateFlashDuration = 300 // ms, kept in sync with style.css by notelist's own golden test

// Empty-state hint block (internal/ui/panel): the first-run "press
// Shift twice" hint and the search-no-matches block share this padding —
// measured as "40dp of breathing room" in the child spec, not from the
// demo video (neither state is shown in it).
const PanelHintPad = 40

// Font stack, closest metric match to SF Pro Text for cap height, x-height
// and leading. Inline bold is weight 600 (SF's bold-in-text), not 700.
//
// The bundled font's own name table declares its family as "Inter
// Variable", not "Inter" (confirmed via fc-scan) — fontconfig matches
// only on the declared name, so the stack must lead with the real name
// or registerBundledFont's registration is silently unused and every
// label falls through to a generic sans-serif.
const (
	FontFamily       = `"Inter Variable", "Adwaita Sans", "Cantarell", sans-serif`
	InlineBoldWeight = 600
)

// Note editor window (internal/ui/editorwindow): an ordinary toplevel
// GtkWindow, not a layer-shell surface, so the compositor tiles and
// focuses it like any other window. Not shown in the demo (the whole
// window model, including its existence, is Ingot's own contract for
// long notes) — chosen so the editor comfortably fits the panel's own
// note-card typography at roughly panel-width-plus-a-margin.
const (
	EditorWidth   = 520
	EditorHeight  = 420
	EditorPadding = 20
	EditorFont    = 15
	EditorLine    = 22

	// EditorSaveDebounceMs is how long the editor waits after the last
	// keystroke before persisting; a close always flushes immediately
	// regardless of how much of this window has elapsed.
	EditorSaveDebounceMs = 400
)

// Tokens that exist in both palettes because a rule or a cairo drawer
// needs them by name. Each one either replaces a literal that used to sit
// inline (so the dark palette has something to override) or is new to the
// selectors added for the dark scheme. The light values are the literals
// that were already shipping — none of them is a new colour decision.
const (
	// DragShadow was the literal box-shadow on .note-card.dragging in
	// style.css.
	DragShadow = "0 6px 16px rgba(0,0,0,.18)"

	// Reject was the literal #E5484D on .composer.reject in style.css.
	Reject = "#E5484D"

	// ToastIconBg and ToastIconTick were cr.SetSourceRGB(0,0,0) and
	// cr.SetSourceRGB(1,1,1) in internal/ui/toast/icon.go — the light
	// toast's filled circle and the tick inscribed in it.
	ToastIconBg   = "#000000"
	ToastIconTick = "#FFFFFF"

	// SelectionBg is the text-selection highlight behind selected
	// characters in the composer, the search entry and the editor. GTK's
	// own `selection` node has no Ingot rule until now, so Adwaita's
	// (near-white on dark) was showing through.
	SelectionBg = "rgba(10,108,255,.28)"

	// OverlayHover and OverlayActive are the tint laid over a surface for
	// the two pointer states on buttons that paint no fill of their own
	// (the overflow button, popover menu items, the empty-state Clear
	// search button). Light darkens, dark lightens — which is exactly why
	// they cannot be one constant.
	OverlayHover  = "rgba(0,0,0,.06)"
	OverlayActive = "rgba(0,0,0,.10)"

	// MenuBg is the GtkPopoverMenu contents fill. Not the card or field
	// surface: a menu floats over the panel rather than sitting in it.
	MenuBg = "#FFFFFF"

	// HighlightBg tints a search match inside a note's rendered body
	// (internal/ui/mdpango's writeHighlighted). It is the accent at 12%
	// alpha, and it is the one colour token that never becomes a CSS
	// custom property: it is emitted as a Pango <span background=...>
	// attribute, which GTK's CSS engine never sees.
	//
	// The 8-digit #RRGGBBAA form is deliberate and verified — Pango's
	// markup parser accepts both #RRGGBB and #RRGGBBAA, so do not
	// "correct" this to 6 digits and lose the alpha.
	HighlightBg = "#0A6CFF1F"
)

// The dark palette. Unlike every light value above, no demo footage
// exists for a dark variant of the original app, so these are Ingot's own
// contract rather than a measurement. They were chosen to preserve the
// light palette's relationships, not to invert it channel by channel:
//
//   - the panel is the darkest surface, and cards and fields are raised
//     above it rather than cut into it;
//   - the card surface stays warm (blue is its lowest channel) and the
//     search/composer field surface stays cool (blue is its highest), the
//     same deliberate warm/cool split the light palette measures;
//   - the selected card stays blue-tinted against both.
const (
	DarkPanelBg = "#1B1C1F"

	// DarkPanelRim carries more alpha than PanelRim's light .40 does work,
	// because in dark it is the only thing drawing the panel's edge. The
	// light panel is separated from the desktop by a drop shadow that
	// reads against a bright wallpaper; against a dark or black desktop
	// that same shadow is invisible (the panel is 1.23:1 against true
	// black), so the rim is the edge.
	DarkPanelRim             = "rgba(255,255,255,.12)"
	DarkPanelShadow          = "0 8px 28px rgba(0,0,0,.55), 0 2px 8px rgba(0,0,0,.35)"
	DarkPanelShadowUnfocused = "0 4px 14px rgba(0,0,0,.35), 0 1px 4px rgba(0,0,0,.18)"

	DarkCardBg         = "#282725"
	DarkCardBgHover    = "#333230"
	DarkCardBgSelected = "#1E3555"
	DarkFieldBg        = "#25272B"

	DarkInk      = "#E9EAEC"
	DarkInkMuted = "#9A9DA3"

	// DarkInkDone is lighter than a straight tonal mirror of InkDone would
	// be, because the mirror (#7C7F85) measured only 3.72:1 on DarkCardBg
	// — below the 4.5:1 body target and worse than the light palette's own
	// 4.77:1 there.
	//
	// This value's measured contrast, and a recorded decision to stop
	// here rather than keep climbing:
	//
	//	DarkPanelBg         5.26:1  pass
	//	DarkCardBg          4.60:1  pass
	//	DarkCardBgHover     3.95:1  fail
	//	DarkCardBgSelected  3.82:1  fail
	//
	// The two failures are the transient states: hover lasts as long as
	// the pointer rests on the row, selected as long as the row is picked.
	// Clearing 4.5:1 on all four needs roughly #9A9DA3, which IS
	// DarkInkMuted — so buying those two states costs the done-versus-
	// muted distinction the palette draws deliberately, and a done note
	// would become indistinguishable from a section header.
	//
	// All-surface AA was never this design's bar: the measured light spec
	// is itself sub-AA here, InkDone on PanelBg being 4.06:1. Colour is
	// also the secondary done cue — internal/ui/widget's drawStrike paints
	// a strikethrough over the whole row, which carries the state on its
	// own and does not depend on contrast at all.
	DarkInkDone = "#8C8F95"

	DarkRule = "rgba(255,255,255,.10)"

	DarkCheckRing    = "#8B8E94"
	DarkAccent       = "#4C8DFF"
	DarkFocusRingDim = "rgba(76,141,255,.45)"

	DarkPlaceholderBorder = "rgba(255,255,255,.14)"
	DarkScrollbarInk      = "rgba(255,255,255,.30)"

	DarkToastDarkBg    = "#232326"
	DarkToastDarkText  = "#FFFFFF"
	DarkToastLightBg   = "rgba(58,58,62,.72)"
	DarkToastLightText = DarkInk

	DarkDragShadow    = "0 6px 18px rgba(0,0,0,.55)"
	DarkReject        = "#FF6369"
	DarkToastIconBg   = "#E9EAEC"
	DarkToastIconTick = "#1B1C1F"
	DarkSelectionBg   = "rgba(76,141,255,.35)"
	DarkOverlayHover  = "rgba(255,255,255,.08)"
	DarkOverlayActive = "rgba(255,255,255,.13)"
	DarkMenuBg        = "#2E2D31"

	// DarkHighlightBg is the dark accent at 20%, not the light palette's
	// 12%: a 12% tint of any colour over #282725 is nearly invisible, so
	// the search match would read as unhighlighted.
	DarkHighlightBg = "#4C8DFF33"
)

// Palette is every colour token of one scheme in a single value. Sizes,
// durations and type are not in here on purpose: they do not change with
// the colour scheme, so style.css remains their only consumer.
//
// Two things read a Palette. style.css reads it indirectly, through the
// CSS custom properties tokens() emits; the cairo DrawFuncs in
// internal/ui/widget, internal/ui/notelist and internal/ui/toast read it
// directly through Colors(), because they paint outside GTK's CSS engine
// and so have no var() to resolve.
type Palette struct {
	PanelBg              string
	PanelRim             string
	PanelShadow          string
	PanelShadowUnfocused string

	CardBg         string
	CardBgHover    string
	CardBgSelected string
	FieldBg        string

	Ink      string
	InkMuted string
	InkDone  string
	Rule     string

	CheckRing    string
	Accent       string
	FocusRingDim string

	PlaceholderBorder string
	ScrollbarInk      string

	ToastDarkBg    string
	ToastDarkText  string
	ToastLightBg   string
	ToastLightText string

	DragShadow    string
	Reject        string
	ToastIconBg   string
	ToastIconTick string
	SelectionBg   string
	OverlayHover  string
	OverlayActive string
	MenuBg        string

	// HighlightBg is deliberately absent from tokens(): it is a Pango
	// attribute value, not a CSS custom property. See its constant.
	HighlightBg string
}

// Light is the measured light palette. Every field is built from the
// constant above it rather than from a fresh literal, so a colour has
// exactly one spelling in this package.
//
// The measured constants are pinned by TestTokensMatchSpec; the tokens
// this change added (DragShadow, Reject, ToastIconBg, ToastIconTick,
// SelectionBg, OverlayHover, OverlayActive, MenuBg, HighlightBg) are
// pinned by TestNewLightTokensMatchSpec instead, because they are not
// part of the measured demo spec that test guards.
var Light = Palette{
	PanelBg:              PanelBg,
	PanelRim:             PanelRim,
	PanelShadow:          PanelShadow,
	PanelShadowUnfocused: PanelShadowUnfocused,

	CardBg:         CardBg,
	CardBgHover:    CardBgHover,
	CardBgSelected: CardBgSelected,
	FieldBg:        FieldBg,

	Ink:      Ink,
	InkMuted: InkMuted,
	InkDone:  InkDone,
	Rule:     Rule,

	CheckRing:    CheckRing,
	Accent:       Accent,
	FocusRingDim: FocusRingDim,

	PlaceholderBorder: PlaceholderBorder,
	ScrollbarInk:      ScrollbarInk,

	ToastDarkBg:    ToastDarkBg,
	ToastDarkText:  ToastDarkText,
	ToastLightBg:   ToastLightBg,
	ToastLightText: ToastLightText,

	DragShadow:    DragShadow,
	Reject:        Reject,
	ToastIconBg:   ToastIconBg,
	ToastIconTick: ToastIconTick,
	SelectionBg:   SelectionBg,
	OverlayHover:  OverlayHover,
	OverlayActive: OverlayActive,
	MenuBg:        MenuBg,
	HighlightBg:   HighlightBg,
}

// Dark is the dark palette described above the Dark* constants.
var Dark = Palette{
	PanelBg:              DarkPanelBg,
	PanelRim:             DarkPanelRim,
	PanelShadow:          DarkPanelShadow,
	PanelShadowUnfocused: DarkPanelShadowUnfocused,

	CardBg:         DarkCardBg,
	CardBgHover:    DarkCardBgHover,
	CardBgSelected: DarkCardBgSelected,
	FieldBg:        DarkFieldBg,

	Ink:      DarkInk,
	InkMuted: DarkInkMuted,
	InkDone:  DarkInkDone,
	Rule:     DarkRule,

	CheckRing:    DarkCheckRing,
	Accent:       DarkAccent,
	FocusRingDim: DarkFocusRingDim,

	PlaceholderBorder: DarkPlaceholderBorder,
	ScrollbarInk:      DarkScrollbarInk,

	ToastDarkBg:    DarkToastDarkBg,
	ToastDarkText:  DarkToastDarkText,
	ToastLightBg:   DarkToastLightBg,
	ToastLightText: DarkToastLightText,

	DragShadow:    DarkDragShadow,
	Reject:        DarkReject,
	ToastIconBg:   DarkToastIconBg,
	ToastIconTick: DarkToastIconTick,
	SelectionBg:   DarkSelectionBg,
	OverlayHover:  DarkOverlayHover,
	OverlayActive: DarkOverlayActive,
	MenuBg:        DarkMenuBg,
	HighlightBg:   DarkHighlightBg,
}

// tokens maps every field of p to the CSS custom property style.css
// declares it under. The names must match style.css's :root block
// exactly — TestLightTokensMatchStylesheet is what actually enforces
// that, and the dark override provider (scheme.go) is generated straight
// from this map, so a typo here would silently stop overriding one token.
func (p Palette) tokens() map[string]string {
	return map[string]string{
		"--panel-bg":               p.PanelBg,
		"--panel-rim":              p.PanelRim,
		"--panel-shadow":           p.PanelShadow,
		"--panel-shadow-unfocused": p.PanelShadowUnfocused,

		"--card-bg":          p.CardBg,
		"--card-bg-hover":    p.CardBgHover,
		"--card-bg-selected": p.CardBgSelected,
		"--field-bg":         p.FieldBg,

		"--ink":       p.Ink,
		"--ink-muted": p.InkMuted,
		"--ink-done":  p.InkDone,
		"--rule":      p.Rule,

		"--check-ring":     p.CheckRing,
		"--accent":         p.Accent,
		"--focus-ring-dim": p.FocusRingDim,

		"--placeholder-border": p.PlaceholderBorder,
		"--scrollbar-ink":      p.ScrollbarInk,

		"--toast-dark-bg":    p.ToastDarkBg,
		"--toast-dark-text":  p.ToastDarkText,
		"--toast-light-bg":   p.ToastLightBg,
		"--toast-light-text": p.ToastLightText,

		"--drag-shadow":     p.DragShadow,
		"--reject":          p.Reject,
		"--toast-icon-bg":   p.ToastIconBg,
		"--toast-icon-tick": p.ToastIconTick,
		"--selection-bg":    p.SelectionBg,
		"--overlay-hover":   p.OverlayHover,
		"--overlay-active":  p.OverlayActive,
		"--menu-bg":         p.MenuBg,
	}
}
