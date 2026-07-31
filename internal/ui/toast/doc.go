// Package toast implements Ingot's two toast widgets, plus the Notifier
// interface every capture/action flow reports through.
//
// These are two different widgets, not one widget in two colours:
//
//   - HUD is the dark global toast (Notifier.Captured): opaque near-black,
//     no icon, its own short-lived overlay layer-shell surface centred on
//     the output. It fires from the global capture chord, which can land
//     while the panel is hidden or unfocused, so it cannot live inside
//     the panel's own surface — a GtkPopover cannot leave that surface,
//     which is why this needs layershell at all.
//   - InPanel is the light in-panel toast (Notifier.Message): translucent
//     vibrancy fill, a filled circle-and-tick icon, a GtkRevealer inside
//     the panel's own GtkOverlay. It fires only for actions that require
//     the panel to already be visible, e.g. Copy as List.
//
// Both share the same 140ms-in/1200ms-hold/120ms-out timing (timing.go)
// and the same replace-not-stack behaviour when a second toast lands
// inside the hold window (sequencer.go) — the fade itself is done inside
// Ingot via style.css's .toast-in/.toast-out classes, not the
// compositor's own surface animations.
//
// When the compositor has no wlr-layer-shell support at all, New falls
// back to org.freedesktop.Notifications for Captured (fallback.go) — only
// as a last resort, since a notification daemon may not exist, cannot be
// made to match Ingot's own design, and piles toasts into the user's
// notification history.
package toast
