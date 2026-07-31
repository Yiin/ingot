package app

import (
	"errors"
	"testing"

	"github.com/Yiin/ingot/internal/setup"
)

func TestChordProbeReason(t *testing.T) {
	tests := []struct {
		name      string
		status    setup.KeyboardStatus
		err       error
		wantEmpty bool
	}{
		{"probe error disables", setup.KeyboardStatus{}, errors.New("boom"), false},
		{"no devices detected disables", setup.KeyboardStatus{Detected: 0, Readable: 0}, nil, false},
		{"detected but unreadable disables", setup.KeyboardStatus{Detected: 2, Readable: 0}, nil, false},
		{"fully readable enables", setup.KeyboardStatus{Detected: 1, Readable: 1}, nil, true},
		{"partially readable still enables", setup.KeyboardStatus{Detected: 2, Readable: 1}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chordProbeReason(tt.status, tt.err)
			if (got == "") != tt.wantEmpty {
				t.Errorf("chordProbeReason(%+v, %v) = %q, want empty=%v", tt.status, tt.err, got, tt.wantEmpty)
			}
		})
	}
}

func TestApp_LastCapturedNote(t *testing.T) {
	st := newTestStore(t)
	projID, err := st.CreateProject("Notes")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	proj, _ := st.Project(projID)
	sec := proj.Sections[0].ID

	a := &App{store: st}

	if got := a.lastCapturedNote(); got != nil {
		t.Fatalf("lastCapturedNote() on an empty section = %+v, want nil", got)
	}

	if _, err := st.AddNote(sec, "one"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if _, err := st.AddNote(sec, "two"); err != nil {
		t.Fatalf("AddNote: %v", err)
	}

	got := a.lastCapturedNote()
	if got == nil || got.Body != "two" {
		t.Errorf("lastCapturedNote() = %+v, want the most recently added note (\"two\")", got)
	}
}
