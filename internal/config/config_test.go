package config

import (
	"testing"
	"time"
)

func TestDefaultHotkey(t *testing.T) {
	got := DefaultHotkey()
	if got.Window != 350*time.Millisecond {
		t.Errorf("DefaultHotkey().Window = %v, want 350ms", got.Window)
	}
	if got.Window != DefaultHotkeyWindow {
		t.Errorf("DefaultHotkey().Window = %v, want DefaultHotkeyWindow (%v)", got.Window, DefaultHotkeyWindow)
	}
}

func TestDefaultCtrlCFallbackIsOff(t *testing.T) {
	if got := DefaultCtrlCFallback(); got.Enabled {
		t.Errorf("DefaultCtrlCFallback().Enabled = true, want false")
	}
}
