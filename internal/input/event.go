package input

import "time"

// Kind classifies a reduced input event. Only KindShift and
// KindPointerButton carry a meaningful Code; KindOther always has Code 0
// so raw key codes for ordinary typing never leave this package.
type Kind int

const (
	KindOther Kind = iota
	KindShift
	KindPointerButton
)

func (k Kind) String() string {
	switch k {
	case KindShift:
		return "shift"
	case KindPointerButton:
		return "pointer_button"
	default:
		return "other"
	}
}

// Event is a reduced, non-identifying view of a raw evdev key or button
// event: enough to detect a double-Shift chord and disqualify it on any
// other key or a mouse click, and nothing more.
type Event struct {
	Kind    Kind
	Code    uint16
	Pressed bool
	At      time.Time
}

// Source streams reduced input events from one or more evdev devices.
// Everything downstream tests against a fake channel implementing this
// interface rather than the real evdev-backed implementation.
type Source interface {
	Events() <-chan Event
	Close() error
}
