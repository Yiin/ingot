package input

import (
	"os"
	"time"

	"github.com/holoplot/go-evdev"
)

// rawDevice is the slice of *evdev.InputDevice this package needs. Tests
// substitute a fake so the hotplug and capability-filtering logic can run
// against a temp directory without root or a real device node.
type rawDevice interface {
	CapableEvents(t evdev.EvType) []evdev.EvCode
	ReadOne() (*evdev.InputEvent, error)
	NonBlock() error
	Close() error
}

var _ rawDevice = (*evdev.InputDevice)(nil)

// openFunc opens the device node at path. Never Grab()s it: multiple
// readers, including the compositor, must keep receiving input.
type openFunc func(path string) (rawDevice, error)

func openReal(path string) (rawDevice, error) {
	return evdev.OpenWithFlags(path, os.O_RDONLY)
}

// hasChordCapability reports whether a device's EV_KEY capability set
// makes it relevant to the double-Shift chord: either Shift key, or the
// left mouse button (which must disqualify the chord even though it
// arrives on a different device node than the keyboard). Never exclude a
// device for exposing pointer capabilities alongside keyboard ones — a
// combo device must still match.
func hasChordCapability(codes []evdev.EvCode) bool {
	for _, c := range codes {
		switch c {
		case evdev.KEY_LEFTSHIFT, evdev.KEY_RIGHTSHIFT, evdev.EvCode(evdev.BTN_LEFT):
			return true
		}
	}
	return false
}

// reduce converts a raw evdev event into the package's non-identifying
// Event, dropping everything that isn't a key/button press or release.
func reduce(ev *evdev.InputEvent) (Event, bool) {
	if ev.Type != evdev.EV_KEY {
		return Event{}, false
	}
	// Value: 0 = release, 1 = press, 2 = autorepeat. Autorepeat carries no
	// information a chord or click detector needs.
	if ev.Value != 0 && ev.Value != 1 {
		return Event{}, false
	}

	at := time.Unix(int64(ev.Time.Sec), int64(ev.Time.Usec)*1000)
	pressed := ev.Value == 1

	switch ev.Code {
	case evdev.KEY_LEFTSHIFT, evdev.KEY_RIGHTSHIFT:
		return Event{Kind: KindShift, Code: uint16(ev.Code), Pressed: pressed, At: at}, true
	case evdev.EvCode(evdev.BTN_LEFT):
		return Event{Kind: KindPointerButton, Code: uint16(ev.Code), Pressed: pressed, At: at}, true
	default:
		return Event{Kind: KindOther, Code: 0, Pressed: pressed, At: at}, true
	}
}
