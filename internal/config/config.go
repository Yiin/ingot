package config

import "time"

// DefaultHotkeyWindow is the maximum gap between two clean Shift taps that
// still counts as a double-tap chord, absent user configuration.
const DefaultHotkeyWindow = 350 * time.Millisecond

// Hotkey holds the user-tunable double-tap chord settings.
type Hotkey struct {
	// Window is the maximum time between the end of the first Shift tap
	// and the start of the second for the pair to still count as a chord.
	Window time.Duration
}

// DefaultHotkey returns the Hotkey settings used until a config file loader
// overrides them.
func DefaultHotkey() Hotkey {
	return Hotkey{Window: DefaultHotkeyWindow}
}

// DefaultPanelToggleBinding is the compositor keybinding Ingot suggests
// for toggling the panel — advisory only, since GTK has no way to
// install a system-wide binding itself; see Config.PanelToggleBinding.
const DefaultPanelToggleBinding = "<Super><Shift>c"

// Config is Ingot's full user-editable configuration, loaded from
// config.toml. Every field defaults to Default's value until config.toml
// overrides it.
type Config struct {
	Hotkey Hotkey
	// PanelToggleBinding is the binding config.toml documents for
	// toggling the panel. Ingot cannot install this itself — there is
	// no portal API for a modifier-only global binding (see the epic's
	// architecture notes) — so this is advisory text `ingot setup`
	// prints for the user to bind in their compositor.
	PanelToggleBinding string
	// Theme names the colour theme. Only "light" exists today; the
	// field exists so a future dark theme has somewhere to plug in
	// without another config.toml schema change.
	Theme string
}

// Default returns Config as it is before any config.toml is read.
func Default() Config {
	return Config{
		Hotkey:             DefaultHotkey(),
		PanelToggleBinding: DefaultPanelToggleBinding,
		Theme:              "light",
	}
}
