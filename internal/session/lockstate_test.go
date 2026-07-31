package session

import "testing"

func TestFakeLockState_InitialValue(t *testing.T) {
	f := NewFakeLockState(true)
	if got := <-f.Locked(); !got {
		t.Errorf("initial Locked() = %v, want true", got)
	}
}

func TestFakeLockState_SetLocked_ReplacesUnreadValue(t *testing.T) {
	f := NewFakeLockState(false)
	f.SetLocked(true)
	f.SetLocked(false)

	// Only the latest value should be observable — SetLocked must not
	// queue every transition.
	got := <-f.Locked()
	if got != false {
		t.Errorf("Locked() = %v, want false (the latest state)", got)
	}
	select {
	case v := <-f.Locked():
		t.Errorf("unexpected second value %v on the channel", v)
	default:
	}
}
