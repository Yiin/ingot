package motion

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

// animationsOverride is a test seam, the same var-override idiom
// internal/ui/toast uses for its own layerShellSupported: nil means "ask
// GTK", non-nil forces EnableAnimations's return value without a live
// display. Production code never writes to it directly — use
// OverrideEnableAnimations.
var animationsOverride *bool

// EnableAnimations mirrors gtk-enable-animations — the single source of
// truth every hand-rolled (non-CSS) animation in internal/ui must check
// before it starts: Animate (tick.go) and FlashClass (class.go) both
// call this internally, so most callers never need to call it
// themselves. It defaults to true whenever the setting can't be read at
// all (no default display, e.g. under go test with no GTK runtime),
// since that is never itself a reason to suppress motion.
func EnableAnimations() bool {
	if animationsOverride != nil {
		return *animationsOverride
	}
	settings := gtk.SettingsGetDefault()
	if settings == nil {
		return true
	}
	enabled, ok := settings.ObjectProperty("gtk-enable-animations").(bool)
	if !ok {
		return true
	}
	return enabled
}

// OverrideEnableAnimations forces EnableAnimations to return enabled
// until the returned restore func is called, for tests that need to
// exercise the disabled path without a live display or real GtkSettings.
// Not safe for concurrent use — GTK code is single-threaded on the main
// loop anyway, and so is every test that calls this.
func OverrideEnableAnimations(enabled bool) (restore func()) {
	prev := animationsOverride
	v := enabled
	animationsOverride = &v
	return func() { animationsOverride = prev }
}
