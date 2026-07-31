package hotkey

import (
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/input"
)

const testWindow = 350 * time.Millisecond

// evdev key codes for LeftShift/RightShift, duplicated here so this test
// package needs no cgo/evdev dependency: only the numeric value matters to
// Detector, which never interprets Code.
const (
	codeLeftShift  uint16 = 42
	codeRightShift uint16 = 54
)

func at(ms int) time.Time {
	return time.Unix(0, 0).Add(time.Duration(ms) * time.Millisecond)
}

func shiftPress(code uint16, ms int) input.Event {
	return input.Event{Kind: input.KindShift, Code: code, Pressed: true, At: at(ms)}
}

func shiftRelease(code uint16, ms int) input.Event {
	return input.Event{Kind: input.KindShift, Code: code, Pressed: false, At: at(ms)}
}

func otherPress(ms int) input.Event {
	return input.Event{Kind: input.KindOther, Pressed: true, At: at(ms)}
}

func otherRelease(ms int) input.Event {
	return input.Event{Kind: input.KindOther, Pressed: false, At: at(ms)}
}

func pointerPress(ms int) input.Event {
	return input.Event{Kind: input.KindPointerButton, Pressed: true, At: at(ms)}
}

// feedAll runs every event through a fresh Detector and reports whether any
// of them fired the chord.
func feedAll(window time.Duration, events []input.Event) bool {
	d := NewDetector(window)
	fired := false
	for _, ev := range events {
		if d.Feed(ev) {
			fired = true
		}
	}
	return fired
}

func TestDetector_VerifiedScenarios(t *testing.T) {
	tests := []struct {
		name   string
		events []input.Event
		want   bool
	}{
		{
			name: "two Shift taps, 150ms gap",
			events: []input.Event{
				shiftPress(codeLeftShift, 0),
				shiftRelease(codeLeftShift, 20),
				shiftPress(codeLeftShift, 170), // 150ms after release
				shiftRelease(codeLeftShift, 190),
			},
			want: true,
		},
		{
			name: "two Shift taps, 747ms gap",
			events: []input.Event{
				shiftPress(codeLeftShift, 0),
				shiftRelease(codeLeftShift, 20),
				shiftPress(codeLeftShift, 767), // 747ms after release
				shiftRelease(codeLeftShift, 787),
			},
			want: false,
		},
		{
			name: "Shift, a, Shift at 273ms — intervening key disarms",
			events: []input.Event{
				shiftPress(codeLeftShift, 0),
				shiftRelease(codeLeftShift, 20),
				otherPress(100),
				otherRelease(110),
				shiftPress(codeLeftShift, 273),
				shiftRelease(codeLeftShift, 293),
			},
			want: false,
		},
		{
			name: "Shift+a twice — other key during hold",
			events: []input.Event{
				shiftPress(codeLeftShift, 0),
				otherPress(10),
				otherRelease(15),
				shiftRelease(codeLeftShift, 20),
				shiftPress(codeLeftShift, 200),
				otherPress(210),
				otherRelease(215),
				shiftRelease(codeLeftShift, 220),
			},
			want: false,
		},
		{
			name: "LeftShift then RightShift, 154ms",
			events: []input.Event{
				shiftPress(codeLeftShift, 0),
				shiftRelease(codeLeftShift, 20),
				shiftPress(codeRightShift, 174), // 154ms after release
				shiftRelease(codeRightShift, 194),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := feedAll(testWindow, tt.events); got != tt.want {
				t.Errorf("fired = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetector_HeldShiftRejected(t *testing.T) {
	d := NewDetector(testWindow)
	events := []input.Event{
		shiftPress(codeLeftShift, 0),
		shiftRelease(codeLeftShift, 450), // held 450ms > 400ms max
	}
	for _, ev := range events {
		if d.Feed(ev) {
			t.Fatal("held tap must not fire")
		}
	}
	// The same detector must have reset to idle, not stuck mid-sequence:
	// feeding a clean pair right after should fire normally.
	rest := []input.Event{
		shiftPress(codeLeftShift, 2000),
		shiftRelease(codeLeftShift, 2020),
		shiftPress(codeLeftShift, 2170),
		shiftRelease(codeLeftShift, 2190),
	}
	fired := false
	for _, ev := range rest {
		if d.Feed(ev) {
			fired = true
		}
	}
	if !fired {
		t.Fatal("detector did not reset after a held-too-long tap")
	}
}

func TestDetector_SecondTapHeldTooLongRejected(t *testing.T) {
	fired := feedAll(testWindow, []input.Event{
		shiftPress(codeLeftShift, 0),
		shiftRelease(codeLeftShift, 20),
		shiftPress(codeLeftShift, 170),
		shiftRelease(codeLeftShift, 600), // second tap held 430ms
	})
	if fired {
		t.Fatal("second tap held past max must not fire")
	}
}

func TestDetector_AutorepeatRejected(t *testing.T) {
	// A second Press of the same Shift key before its Release models
	// EV_KEY autorepeat (value == 2): it must disqualify the in-progress
	// tap rather than being mistaken for a second, independent tap.
	events := []input.Event{
		shiftPress(codeLeftShift, 0),
		shiftPress(codeLeftShift, 50), // autorepeat while still held
		shiftRelease(codeLeftShift, 100),
		shiftPress(codeLeftShift, 150),
		shiftRelease(codeLeftShift, 180),
	}
	if fired := feedAll(testWindow, events); fired {
		t.Fatal("autorepeat during the first tap must not fire")
	}
}

func TestDetector_AutorepeatDuringSecondTapRejected(t *testing.T) {
	events := []input.Event{
		shiftPress(codeLeftShift, 0),
		shiftRelease(codeLeftShift, 20),
		shiftPress(codeLeftShift, 170),
		shiftPress(codeLeftShift, 220), // autorepeat during second tap
		shiftRelease(codeLeftShift, 260),
	}
	if fired := feedAll(testWindow, events); fired {
		t.Fatal("autorepeat during the second tap must not fire")
	}
}

func TestDetector_PointerButtonDuringHoldDisarms(t *testing.T) {
	events := []input.Event{
		shiftPress(codeLeftShift, 0),
		pointerPress(10),
		shiftRelease(codeLeftShift, 20),
	}
	if fired := feedAll(testWindow, events); fired {
		t.Fatal("pointer button during hold must not fire")
	}
}

func TestDetector_PointerButtonBetweenTapsDisarms(t *testing.T) {
	events := []input.Event{
		shiftPress(codeLeftShift, 0),
		shiftRelease(codeLeftShift, 20),
		pointerPress(50),
		shiftPress(codeLeftShift, 170),
		shiftRelease(codeLeftShift, 190),
	}
	if fired := feedAll(testWindow, events); fired {
		t.Fatal("pointer button between taps must not fire")
	}
}

func TestDetector_IgnoresStrayReleaseWhenIdle(t *testing.T) {
	d := NewDetector(testWindow)
	if d.Feed(shiftRelease(codeLeftShift, 0)) {
		t.Fatal("a bare release must never fire")
	}
	if d.Feed(otherRelease(1)) {
		t.Fatal("a bare non-Shift release must never fire")
	}
	// The same detector must still be idle and ready for a clean pair.
	rest := []input.Event{
		shiftPress(codeLeftShift, 100),
		shiftRelease(codeLeftShift, 120),
		shiftPress(codeLeftShift, 270),
		shiftRelease(codeLeftShift, 290),
	}
	fired := false
	for _, ev := range rest {
		if d.Feed(ev) {
			fired = true
		}
	}
	if !fired {
		t.Fatal("detector did not remain idle-ready after stray releases")
	}
}

func TestDetector_IgnoresStrayReleaseWhileDown(t *testing.T) {
	// A release of some other kind arriving mid-hold (never produced by
	// the real evdev reduction, but the state machine must not
	// misinterpret it) must not disarm the tap in progress.
	events := []input.Event{
		shiftPress(codeLeftShift, 0),
		otherRelease(10),
		shiftRelease(codeLeftShift, 20),
		shiftPress(codeLeftShift, 170),
		otherRelease(180),
		shiftRelease(codeLeftShift, 190),
	}
	if fired := feedAll(testWindow, events); !fired {
		t.Fatal("a stray release mid-hold must not disarm a clean tap")
	}
}

func TestDetector_IgnoresStrayReleaseWhileWaiting(t *testing.T) {
	events := []input.Event{
		shiftPress(codeLeftShift, 0),
		shiftRelease(codeLeftShift, 20),
		otherRelease(50),
		shiftPress(codeLeftShift, 170),
		shiftRelease(codeLeftShift, 190),
	}
	if fired := feedAll(testWindow, events); !fired {
		t.Fatal("a stray release between taps must not disarm the sequence")
	}
}

func TestDetector_ConfiguredWindowIsHonored(t *testing.T) {
	// A wider configured window admits a gap that the default would reject.
	events := []input.Event{
		shiftPress(codeLeftShift, 0),
		shiftRelease(codeLeftShift, 20),
		shiftPress(codeLeftShift, 767), // 747ms gap
		shiftRelease(codeLeftShift, 787),
	}
	if fired := feedAll(800*time.Millisecond, events); !fired {
		t.Fatal("a wider configured window should admit a 747ms gap")
	}
}

func TestDetector_FeedAllocatesNothing(t *testing.T) {
	d := NewDetector(testWindow)
	events := []input.Event{
		shiftPress(codeLeftShift, 0),
		shiftRelease(codeLeftShift, 20),
		shiftPress(codeLeftShift, 170),
		shiftRelease(codeLeftShift, 190),
	}
	i := 0
	allocs := testing.AllocsPerRun(1000, func() {
		ev := events[i%len(events)]
		i++
		d.Feed(ev)
	})
	if allocs != 0 {
		t.Fatalf("Feed allocated %v times per call, want 0", allocs)
	}
}
