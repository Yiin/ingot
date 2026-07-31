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

// Font stack, closest metric match to SF Pro Text for cap height, x-height
// and leading. Inline bold is weight 600 (SF's bold-in-text), not 700.
const (
	FontFamily       = `"Inter", "Adwaita Sans", "Cantarell", sans-serif`
	InlineBoldWeight = 600
)
