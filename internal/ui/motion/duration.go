package motion

import "time"

// Row insert (a capture or composer commit landing in the list): height
// 0->h, opacity 0->1, translateY(-4px)->0, on EmphasizedDecelerate. Rows
// below the inserted one slide on the same curve. Implemented as
// style.css's ingot-row-in @keyframes (internal/ui/notelist);
// RowInsertDuration must stay equal to that rule's own duration — see
// internal/ui/notelist's TestInsertAnimDurationMatchesCSS.
const RowInsertDuration = 180 * time.Millisecond

// Row remove (delete, Clear Done): height h->0, opacity 1->0, on
// Decelerate. Not yet wired to a real removal animation anywhere —
// internal/ui/notelist currently removes a spliced-out row's widget
// immediately, with no exit animation. Kept here so whichever child adds
// one starts from the measured value instead of guessing.
const RowRemoveDuration = 140 * time.Millisecond

// Hover fill: the note card's background brightens on HoverInDuration
// (entering hover) and settles back on the longer HoverOutDuration
// (leaving hover) — asymmetric on purpose, since a quick fade back out
// avoids flicker when the pointer crosses the gap between two cards.
// Both on EaseOut. Implemented as style.css's .note-card:hover transition.
const (
	HoverInDuration  = 90 * time.Millisecond
	HoverOutDuration = 140 * time.Millisecond
)

// Focus ring: opacity 0->1 and width 0->2px (theme.FocusRing), on
// EaseOut. The ring's position must never animate — only its own
// presence — so moving focus between two widets reads as instant.
// Implemented as style.css's *:focus-visible transition.
const FocusRingDuration = 120 * time.Millisecond

// Selection fill: the note card's selected background crossfades in on
// EaseOut. Implemented as style.css's .note-card.selected transition.
const SelectionFillDuration = 120 * time.Millisecond

// Checkbox tick: CheckboxFillDuration of ring fill on EaseOut, then the
// tick stroke draws over CheckboxTickDuration on EmphasizedDecelerate,
// starting CheckboxTickOverlap before the fill finishes.
// CheckboxTotalDuration is the sum minus the overlap — 220ms total, per
// spec. Hand-rolled via cairo + AddTickCallback, not CSS — see
// internal/ui/widget's checkbox.go/easing.go, which predates this
// package and implements the identical numbers independently (fillEase,
// tickEase, fillDuration, tickDuration, tickOverlap, checkDuration).
const (
	CheckboxFillDuration  = 120 * time.Millisecond
	CheckboxTickDuration  = 140 * time.Millisecond
	CheckboxTickOverlap   = 40 * time.Millisecond
	CheckboxTotalDuration = CheckboxFillDuration + CheckboxTickDuration - CheckboxTickOverlap
)

// Strikethrough: a left-to-right wipe over the done note's text,
// synchronised with the checkbox tick (both start together and both land
// on CheckboxTotalDuration/StrikethroughDuration respectively — the wipe
// finishes 20ms after the tick). Hand-rolled via cairo + AddTickCallback
// — see internal/ui/widget/noterow.go's strikeDuration, which predates
// this package and implements the identical number independently.
const StrikethroughDuration = 200 * time.Millisecond

// Composer growth past its fixed 3-line height: height only, ease-out. A
// natural-size change, not a CSS property, so it needs AddTickCallback
// driving GtkScrolledWindow.SetMinContentHeight rather than a CSS
// transition — see internal/ui/composer's animateHeightTo, which
// predates this package and implements the identical duration
// independently (growthMs) on its own ease-out-shaped cubic
// (1-(1-t)^3), not EaseOut's cubic-bezier curve; not retouched here for
// the same reason as the strikethrough/EaseOut note in easing.go.
const ComposerGrowthDuration = 120 * time.Millisecond

// Toast in / hold / out: ToastInDuration of scale(.94)->1 fade-in, held
// for ToastHoldDuration, then ToastOutDuration of translateY(0->4px)
// fade-out. Implemented as style.css's .toast-in/.toast-out @keyframes
// (internal/ui/toast) — see internal/ui/toast/timing.go, which predates
// this package and implements the identical durations independently
// (FadeInDuration, HoldDuration, FadeOutDuration). Its shipped curves
// (cubic-bezier(0,0,.58,1) in, plain ease out) are close to but not
// exactly EmphasizedDecelerate/EaseIn as originally measured; left
// alone rather than retouched, since toast.go's fade timing is
// deliberately coordinated with its own GtkRevealer crossfade (see
// InPanel.exit's own comment) and not worth the risk of a cosmetic
// re-tune.
const (
	ToastInDuration   = 140 * time.Millisecond
	ToastHoldDuration = 1200 * time.Millisecond
	ToastOutDuration  = 120 * time.Millisecond
)

// Context menu open / dismiss: ContextMenuOpenDuration of
// scale(.97)->1 on EaseOut, anchored at the pointer corner;
// ContextMenuDismissDuration of an opacity-only fade. Not yet wired to
// anything: internal/ui/menus builds GtkPopoverMenu content, and GTK4's
// GtkPopover has no built-in open/close animation of its own to hook —
// reaching this value needs either a hand-rolled CSS transition on the
// popover's own "popover" CSS node or an Animate-driven one. Kept here
// as the measured reference value for whichever child wires it.
const (
	ContextMenuOpenDuration    = 90 * time.Millisecond
	ContextMenuDismissDuration = 140 * time.Millisecond
)

// Panel show / hide: PanelShowDuration in on EmphasizedDecelerate
// (opacity plus translateX(+16px)->0), PanelHideDuration out reversed. Per the child
// spec, this must be a GtkRevealer around the panel's own content — the
// Wayland surface itself commits at final size and maps/unmaps
// instantly (internal/layershell), never animated. Not yet wired: no
// caller shows/hides the panel yet (that lands with copper-l2z.30's
// end-to-end wiring). Reveal is this package's helper for that wiring to
// use.
const (
	PanelShowDuration = 200 * time.Millisecond
	PanelHideDuration = 150 * time.Millisecond
)

// ScrollToInsertedDuration is how long the list takes to scroll a freshly
// inserted row into view, on EaseInOut — and only when that row lands
// outside the current viewport; a row that inserts already on-screen
// must trigger zero scroll animations. See ShouldScroll (scroll.go) for
// the on/off-screen decision this package leaves to its own pure
// function rather than trusting gtk.ListView.ScrollTo to already make
// it, since that behaviour is not part of GTK's documented contract.
const ScrollToInsertedDuration = 240 * time.Millisecond

// Overlay scrollbar: ScrollbarInDuration fade-in while scrolling,
// ScrollbarHoldDuration held after the last scroll event, then
// ScrollbarOutDuration fade-out — all on Ease. Implemented as style.css's
// .ingot-scrollbar/.ingot-scrollbar.scrolling opacity transitions plus
// internal/ui/notelist/scrollbar.go's own hold timer (scrollbarHold),
// which predates this package and implements the identical numbers
// independently.
const (
	ScrollbarInDuration   = 80 * time.Millisecond
	ScrollbarHoldDuration = 700 * time.Millisecond
	ScrollbarOutDuration  = 300 * time.Millisecond
)
