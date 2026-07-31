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

// CtrlCFallback holds the user-tunable synthetic Ctrl+C clipboard
// capture settings — the fallback for apps that fill only CLIPBOARD,
// never PRIMARY, on selection.
type CtrlCFallback struct {
	// Enabled turns the fallback on. It types a keystroke into whatever
	// application currently has focus, which is invasive enough that it
	// must be an explicit opt-in rather than a sensible default.
	Enabled bool
}

// DefaultCtrlCFallback returns the CtrlCFallback settings used until a
// config file loader overrides them: off.
func DefaultCtrlCFallback() CtrlCFallback {
	return CtrlCFallback{Enabled: false}
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
	// CtrlCFallback holds the opt-in synthetic Ctrl+C clipboard capture
	// settings. Off unless config.toml explicitly enables it.
	CtrlCFallback CtrlCFallback
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
	// Keys holds per-action accelerator overrides from config.toml's
	// [keys] section, keyed by keymap.Entry.Action (e.g. "mark-done" =
	// "<Control>space"). Validating each key against keymap.ByAction
	// happens in internal/app, not here: keymap lives under internal/ui
	// and pulls in cgo, and this package stays free of that so it keeps
	// building and testing without a display, the same as every other
	// internal/store-adjacent package.
	Keys map[string]string
}

// Default returns Config as it is before any config.toml is read.
func Default() Config {
	return Config{
		Hotkey:             DefaultHotkey(),
		CtrlCFallback:      DefaultCtrlCFallback(),
		PanelToggleBinding: DefaultPanelToggleBinding,
		Theme:              "light",
		Keys:               map[string]string{},
	}
}
