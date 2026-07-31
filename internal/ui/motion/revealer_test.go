package motion

import "testing"

type fakeRevealable struct {
	duration uint
	revealed bool
}

func (f *fakeRevealable) SetTransitionDuration(msec uint) { f.duration = msec }
func (f *fakeRevealable) SetRevealChild(revealChild bool) { f.revealed = revealChild }

func TestRevealShowUsesShowDuration(t *testing.T) {
	r := &fakeRevealable{}
	Reveal(r, true, PanelShowDuration, PanelHideDuration)

	if !r.revealed {
		t.Error("SetRevealChild(true) was not called")
	}
	if want := uint(PanelShowDuration.Milliseconds()); r.duration != want {
		t.Errorf("transition duration = %dms, want %dms (PanelShowDuration)", r.duration, want)
	}
}

func TestRevealHideUsesHideDuration(t *testing.T) {
	r := &fakeRevealable{revealed: true}
	Reveal(r, false, PanelShowDuration, PanelHideDuration)

	if r.revealed {
		t.Error("SetRevealChild(false) was not called")
	}
	if want := uint(PanelHideDuration.Milliseconds()); r.duration != want {
		t.Errorf("transition duration = %dms, want %dms (PanelHideDuration)", r.duration, want)
	}
}

func TestRevealZeroDuration(t *testing.T) {
	r := &fakeRevealable{}
	Reveal(r, true, 0, 0)
	if r.duration != 0 {
		t.Errorf("transition duration = %dms, want 0", r.duration)
	}
}
