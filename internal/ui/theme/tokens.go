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

// Toast: the dark global HUD and the light in-panel toast (built in
// copper-l2z.24 on top of the .toast-dark / .toast-light classes defined
// here).
const (
	ToastHeight = 34
	ToastRadius = 17
	ToastPadX   = 16
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
