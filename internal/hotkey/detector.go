package hotkey

import (
	"time"

	"github.com/Yiin/ingot/internal/input"
)

// maxTapHold is the longest a single Shift press may be held and still
// count as a clean tap. Fixed, unlike Window: a hold this long can no
// longer be a "tap" by definition, regardless of user configuration.
const maxTapHold = 400 * time.Millisecond

type state int

const (
	stateIdle state = iota
	stateFirstDown
	stateWaiting
	stateSecondDown
)

// Detector recognizes two clean Shift taps within Window as a double-tap
// chord. It holds only value-typed fields, so Feed and reset never
// allocate.
type Detector struct {
	window time.Duration

	state          state
	firstPressAt   time.Time
	firstReleaseAt time.Time
	secondPressAt  time.Time
}

// NewDetector returns a Detector that fires on two clean Shift taps within
// window. Callers source window from config.Hotkey.Window (or
// config.DefaultHotkeyWindow).
func NewDetector(window time.Duration) *Detector {
	return &Detector{window: window}
}

// Feed advances the state machine with one reduced input event and reports
// whether this event completed a double-tap chord. Disqualifying signals —
// any non-Shift key press, any pointer-button press, a Shift held past
// maxTapHold, autorepeat (a second Shift press before its release), or a
// gap between taps past window — reset the detector to idle without
// firing.
func (d *Detector) Feed(ev input.Event) (fired bool) {
	switch d.state {
	case stateIdle:
		return d.feedIdle(ev)
	case stateFirstDown:
		return d.feedDown(ev, &d.firstPressAt, &d.firstReleaseAt, stateWaiting)
	case stateWaiting:
		return d.feedWaiting(ev)
	default: // stateSecondDown
		return d.feedSecondDown(ev)
	}
}

func (d *Detector) feedIdle(ev input.Event) bool {
	if ev.Kind == input.KindShift && ev.Pressed {
		d.state = stateFirstDown
		d.firstPressAt = ev.At
	}
	return false
}

// feedDown handles the shared shape of "waiting for the release of the
// currently-down Shift key": a clean release within maxTapHold advances to
// next on success; anything else that is itself a press (autorepeat of
// this key, a different key, or a pointer button) disqualifies the whole
// sequence. A release does not disqualify.
func (d *Detector) feedDown(ev input.Event, pressAt, releaseAt *time.Time, next state) bool {
	if ev.Kind == input.KindShift && !ev.Pressed {
		if ev.At.Sub(*pressAt) > maxTapHold {
			d.reset()
			return false
		}
		*releaseAt = ev.At
		d.state = next
		return false
	}
	if ev.Pressed {
		d.reset()
	}
	return false
}

func (d *Detector) feedWaiting(ev input.Event) bool {
	if ev.Kind == input.KindShift && ev.Pressed {
		if ev.At.Sub(d.firstReleaseAt) > d.window {
			d.reset()
			return false
		}
		d.state = stateSecondDown
		d.secondPressAt = ev.At
		return false
	}
	if ev.Pressed {
		d.reset()
	}
	return false
}

func (d *Detector) feedSecondDown(ev input.Event) bool {
	if ev.Kind == input.KindShift && !ev.Pressed {
		fired := ev.At.Sub(d.secondPressAt) <= maxTapHold
		d.reset()
		return fired
	}
	if ev.Pressed {
		d.reset()
	}
	return false
}

func (d *Detector) reset() {
	*d = Detector{window: d.window}
}
