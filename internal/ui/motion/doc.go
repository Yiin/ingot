// Package motion centralises every duration and easing curve measured for
// Ingot's motion pass, plus the two reusable primitives every consumer
// needs to build on: a CSS-class-toggle helper (class.go) and a
// gtk.Widget.AddTickCallback helper (tick.go). See the child spec
// (copper-l2z.29) for the full per-element table this package encodes.
//
// Reduced motion has two different mechanisms, deliberately not unified
// into one code path:
//
//   - Anything driven by a CSS `transition` or `@keyframes` rule (row
//     insert, hover, selection, focus ring, the toast fades, the overlay
//     scrollbar) or by a GtkRevealer/GtkPopover's own built-in transition
//     is already gated by GTK itself: gtk-enable-animations off collapses
//     every such animation to its end state with no Go-side code at all.
//     internal/ui/theme's style.css is the source of truth for these; this
//     package's constants exist to keep Go-side timers (e.g. the class
//     removal deadline in a "just-inserted" style bookkeeping) in sync
//     with those CSS values, and to be the one place a duration is looked
//     up rather than re-measured per package.
//   - Anything hand-rolled outside CSS — a cairo draw driven by
//     AddTickCallback, or any other manual per-frame state machine — does
//     NOT get gtk-enable-animations for free and must call
//     EnableAnimations itself. internal/ui/widget's checkbox fill/tick and
//     row strikethrough, and internal/ui/composer's height growth, are the
//     three existing cases; Animate (tick.go) is the helper any future one
//     should use.
package motion
