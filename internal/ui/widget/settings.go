package widget

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

// enableAnimations mirrors gtk-enable-animations, honoured per the
// checkbox's own spec. It defaults to true whenever the setting can't be
// read at all (no default display), since that is never itself a reason
// to suppress motion.
func enableAnimations() bool {
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
