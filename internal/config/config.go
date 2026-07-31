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
