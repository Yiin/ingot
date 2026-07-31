// Package clipboard writes formatted note lists to the Wayland CLIPBOARD
// selection.
//
// It shells out to wl-copy, which talks ext-data-control-v1 or
// wlr-data-control-v1 directly and therefore needs neither keyboard focus
// nor an input serial — unlike GDK's own clipboard API. wl-copy forks a
// small background server that owns the copied text after the invoking
// process exits, which is what keeps a "Copy as List" result pasteable
// after Ingot quits (verified: killing wl-copy --foreground makes the
// clipboard read "Nothing is copied", while the default forking mode
// survives; only one such server exists at a time, since each new
// selection owner replaces the last).
//
// When wl-copy is not on PATH, callers may supply a Fallback that sets the
// clipboard some other way. This package deliberately does not import GDK
// itself: only internal/ui and internal/layershell are allowed to use cgo,
// so a GDK-backed fallback (gdk.Display.Clipboard().SetText) must be
// constructed and injected by internal/ui. That fallback is weaker than
// wl-copy: GDK issues set_selection with serial 0 (observed under
// WAYLAND_DEBUG), which Hyprland accepts leniently but a stricter
// compositor such as KWin may reject. Treat it as a degraded mode, not a
// portable guarantee.
package clipboard
