package theme

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

// TestResolveScheme walks the whole precedence table. resolveScheme is a
// pure function precisely so this needs neither a display nor a session
// bus: the three IO wrappers around it are the only parts that do.
func TestResolveScheme(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		portal   portalPref
		gtkTheme string
		want     Scheme
	}{
		{"env dark pin beats a light portal", "dark", portalPrefersLight, "Adwaita", SchemeDark},
		{"env light pin beats a dark portal", "light", portalPrefersDark, "Adwaita-dark", SchemeLight},
		{"env pin is case-insensitive", "DARK", portalPrefersLight, "Adwaita", SchemeDark},
		{"env auto defers to the portal", "auto", portalPrefersDark, "Adwaita", SchemeDark},
		{"an unrecognised env value defers to the portal", "sepia", portalPrefersDark, "Adwaita", SchemeDark},

		{"portal 1 is dark", "", portalPrefersDark, "Adwaita", SchemeDark},
		{"portal 2 is light", "", portalPrefersLight, "Adwaita-dark", SchemeLight},
		{"portal 0 falls through to the theme name", "", portalNoPreference, "Adwaita-dark", SchemeDark},
		{"no portal falls through to the theme name", "", portalUnavailable, "Adwaita-dark", SchemeDark},

		{"a -dark suffix is dark", "", portalUnavailable, "Yaru-blue-dark", SchemeDark},
		{"the -dark suffix match is case-insensitive", "", portalUnavailable, "Adwaita-Dark", SchemeDark},
		{"dark anywhere but the suffix is not dark", "", portalUnavailable, "darkly-light", SchemeLight},

		{"nothing at all is light", "", portalUnavailable, "", SchemeLight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveScheme(tt.env, tt.portal, tt.gtkTheme); got != tt.want {
				t.Errorf("resolveScheme(%q, %d, %q) = %v, want %v", tt.env, tt.portal, tt.gtkTheme, got, tt.want)
			}
		})
	}
}

// TestPinnedScheme guards the half of the env contract resolveScheme
// cannot show on its own: Watch consults pinnedScheme to decide whether
// to subscribe at all, so "auto" and a typo must both report unpinned.
func TestPinnedScheme(t *testing.T) {
	tests := []struct {
		env    string
		want   Scheme
		pinned bool
	}{
		{"dark", SchemeDark, true},
		{"light", SchemeLight, true},
		{" Dark ", SchemeDark, true},
		{"auto", SchemeLight, false},
		{"", SchemeLight, false},
		{"nonsense", SchemeLight, false},
	}
	for _, tt := range tests {
		got, pinned := pinnedScheme(tt.env)
		if pinned != tt.pinned || (pinned && got != tt.want) {
			t.Errorf("pinnedScheme(%q) = (%v, %v), want (%v, %v)", tt.env, got, pinned, tt.want, tt.pinned)
		}
	}
}

// TestDecodePortalPref covers the shapes the portal answers in. ReadOne
// is declared to return a plain variant but implementations nest it one
// deeper, and anything unreadable must report unavailable rather than
// guess a scheme.
func TestDecodePortalPref(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want portalPref
		ok   bool
	}{
		{"bare uint32", uint32(1), portalPrefersDark, true},
		{"variant", dbus.MakeVariant(uint32(2)), portalPrefersLight, true},
		{"variant in variant", dbus.MakeVariant(dbus.MakeVariant(uint32(0))), portalNoPreference, true},
		{"out of range", uint32(7), portalUnavailable, false},
		{"wrong type", "dark", portalUnavailable, false},
		{"nil", nil, portalUnavailable, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodePortalPref(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Errorf("decodePortalPref(%#v) = (%d, %v), want (%d, %v)", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

// TestDecodeSettingChanged checks Watch only reacts to the appearance
// namespace's own color-scheme key. The portal multiplexes every
// namespace's settings onto this one signal.
func TestDecodeSettingChanged(t *testing.T) {
	changed := func(body ...any) *dbus.Signal {
		return &dbus.Signal{Name: portalChanged, Body: body}
	}

	tests := []struct {
		name string
		sig  *dbus.Signal
		want portalPref
		ok   bool
	}{
		{"the colour scheme changed", changed(appearanceNamespace, colorSchemeKey, dbus.MakeVariant(uint32(1))), portalPrefersDark, true},
		{"another key in the same namespace", changed(appearanceNamespace, "accent-color", dbus.MakeVariant(uint32(1))), portalUnavailable, false},
		{"another namespace entirely", changed("org.gnome.desktop.interface", colorSchemeKey, dbus.MakeVariant(uint32(1))), portalUnavailable, false},
		{"a truncated body", changed(appearanceNamespace, colorSchemeKey), portalUnavailable, false},
		{"some other signal", &dbus.Signal{Name: "org.freedesktop.DBus.NameOwnerChanged"}, portalUnavailable, false},
		{"nil", nil, portalUnavailable, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodeSettingChanged(tt.sig)
			if got != tt.want || ok != tt.ok {
				t.Errorf("decodeSettingChanged = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

// The env pin's other half — that a pinned scheme genuinely ignores a
// live portal change — cannot be proven without emitting a real signal,
// so it lives in scheme_portal_integration_test.go as
// TestWatchIgnoresThePortalWhenPinned alongside TestWatchFollowsThePortal.
// A unit test here could only assert that Watch returns, which stays true
// even if the pin check is deleted.
