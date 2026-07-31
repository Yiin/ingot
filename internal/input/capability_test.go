package input

import (
	"testing"

	"github.com/holoplot/go-evdev"
)

func TestHasChordCapability(t *testing.T) {
	tests := []struct {
		name  string
		codes []evdev.EvCode
		want  bool
	}{
		{
			name:  "left shift only",
			codes: []evdev.EvCode{evdev.KEY_A, evdev.KEY_LEFTSHIFT, evdev.KEY_Z},
			want:  true,
		},
		{
			name:  "right shift only",
			codes: []evdev.EvCode{evdev.KEY_RIGHTSHIFT},
			want:  true,
		},
		{
			name:  "btn left only, a mouse",
			codes: []evdev.EvCode{evdev.EvCode(evdev.BTN_LEFT), evdev.EvCode(evdev.BTN_RIGHT)},
			want:  true,
		},
		{
			name:  "combo device: both shift and btn_left, must not be excluded on pointer capability",
			codes: []evdev.EvCode{evdev.KEY_LEFTSHIFT, evdev.KEY_RIGHTSHIFT, evdev.EvCode(evdev.BTN_LEFT)},
			want:  true,
		},
		{
			name:  "ordinary keyboard keys, no shift",
			codes: []evdev.EvCode{evdev.KEY_A, evdev.KEY_B, evdev.KEY_ENTER},
			want:  false,
		},
		{
			name:  "empty capability set",
			codes: nil,
			want:  false,
		},
		{
			name:  "power button, unrelated device",
			codes: []evdev.EvCode{evdev.KEY_POWER},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasChordCapability(tt.codes); got != tt.want {
				t.Errorf("hasChordCapability(%v) = %v, want %v", tt.codes, got, tt.want)
			}
		})
	}
}
