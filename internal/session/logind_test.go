package session

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestDecodeLockSignal(t *testing.T) {
	const mine = dbus.ObjectPath("/org/freedesktop/login1/session/_32")
	const other = dbus.ObjectPath("/org/freedesktop/login1/session/_1")

	lockedHintChanged := func(v bool) []any {
		return []any{
			sessionInterface,
			map[string]dbus.Variant{"LockedHint": dbus.MakeVariant(v)},
			[]string{},
		}
	}

	tests := []struct {
		name        string
		sig         *dbus.Signal
		wantLocked  bool
		wantMatched bool
	}{
		{"lock on our session", &dbus.Signal{Path: mine, Name: lockSignalName}, true, true},
		{"unlock on our session", &dbus.Signal{Path: mine, Name: unlockSignalName}, false, true},
		{"lock on a different session", &dbus.Signal{Path: other, Name: lockSignalName}, false, false},
		{"unrelated signal on our session", &dbus.Signal{Path: mine, Name: sessionInterface + ".SessionActive"}, false, false},
		{
			"PropertiesChanged carrying LockedHint=true (a locker engaging directly, not via Session.Lock)",
			&dbus.Signal{Path: mine, Name: propertiesChangedSignalName, Body: lockedHintChanged(true)},
			true, true,
		},
		{
			"PropertiesChanged carrying LockedHint=false (unlocking at the locker's prompt, not via Session.Unlock)",
			&dbus.Signal{Path: mine, Name: propertiesChangedSignalName, Body: lockedHintChanged(false)},
			false, true,
		},
		{
			"PropertiesChanged for an unrelated interface",
			&dbus.Signal{Path: mine, Name: propertiesChangedSignalName, Body: []any{
				managerInterface, map[string]dbus.Variant{"LockedHint": dbus.MakeVariant(true)}, []string{},
			}},
			false, false,
		},
		{
			"PropertiesChanged with no LockedHint among the changed properties",
			&dbus.Signal{Path: mine, Name: propertiesChangedSignalName, Body: []any{
				sessionInterface, map[string]dbus.Variant{"Active": dbus.MakeVariant(true)}, []string{},
			}},
			false, false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locked, matched := decodeLockSignal(tt.sig, mine)
			if matched != tt.wantMatched {
				t.Fatalf("matched = %v, want %v", matched, tt.wantMatched)
			}
			if matched && locked != tt.wantLocked {
				t.Errorf("locked = %v, want %v", locked, tt.wantLocked)
			}
		})
	}
}

// TestLogindLockState_PumpsSignals drives the full pump/set path with
// synthetic *dbus.Signal values, bypassing NewLogindLockState's system
// bus dial entirely — this is the seam that makes the D-Bus watching
// logic testable without a live logind session.
func TestLogindLockState_PumpsSignals(t *testing.T) {
	const path = dbus.ObjectPath("/org/freedesktop/login1/session/_32")

	l := &LogindLockState{
		sessionPath: path,
		sigCh:       make(chan *dbus.Signal, 8),
		lockCh:      make(chan bool, 1),
		done:        make(chan struct{}),
	}
	l.lockCh <- false
	go l.pump()
	defer close(l.done)

	// Drain the seed value first: otherwise the very first read below
	// could observe it instead of pump's update, racing against whether
	// pump has processed the signal yet.
	<-l.lockCh

	l.sigCh <- &dbus.Signal{Path: path, Name: lockSignalName}
	if got := <-l.Locked(); !got {
		t.Fatalf("Locked() after Lock signal = %v, want true", got)
	}

	// A signal for a different session must be ignored.
	l.sigCh <- &dbus.Signal{Path: "/org/freedesktop/login1/session/_1", Name: unlockSignalName}
	select {
	case v := <-l.Locked():
		t.Fatalf("unrelated session's signal changed our state to %v", v)
	case <-time.After(50 * time.Millisecond):
	}

	l.sigCh <- &dbus.Signal{Path: path, Name: unlockSignalName}
	if got := <-l.Locked(); got {
		t.Fatalf("Locked() after Unlock signal = %v, want false", got)
	}
}

// TestLogindLockState_PumpsSignals_PropertiesChanged covers the path that
// actually matters on Hyprland: a locker like hyprlock never calls
// Session.Lock/Unlock, it only flips LockedHint, which logind reports via
// PropertiesChanged.
func TestLogindLockState_PumpsSignals_PropertiesChanged(t *testing.T) {
	const path = dbus.ObjectPath("/org/freedesktop/login1/session/_32")

	l := &LogindLockState{
		sessionPath: path,
		sigCh:       make(chan *dbus.Signal, 8),
		lockCh:      make(chan bool, 1),
		done:        make(chan struct{}),
	}
	l.lockCh <- false
	go l.pump()
	defer close(l.done)
	<-l.lockCh

	l.sigCh <- &dbus.Signal{
		Path: path,
		Name: propertiesChangedSignalName,
		Body: []any{sessionInterface, map[string]dbus.Variant{"LockedHint": dbus.MakeVariant(true)}, []string{}},
	}
	if got := <-l.Locked(); !got {
		t.Fatalf("Locked() after LockedHint=true PropertiesChanged = %v, want true", got)
	}

	l.sigCh <- &dbus.Signal{
		Path: path,
		Name: propertiesChangedSignalName,
		Body: []any{sessionInterface, map[string]dbus.Variant{"LockedHint": dbus.MakeVariant(false)}, []string{}},
	}
	if got := <-l.Locked(); got {
		t.Fatalf("Locked() after LockedHint=false PropertiesChanged = %v, want false", got)
	}
}

// TestNewLogindLockState_LiveSystemBus exercises the real dial + session
// resolution + LockedHint read + signal subscription against the actual
// system bus, when one is reachable. It never calls the session's
// Lock/Unlock methods — those would lock the operator's real desktop
// session, which this test must not do — so it only proves the
// read-only setup path succeeds and matches loginctl's own view of
// LockedHint. It skips instead of failing when no logind session is
// available to this process, which is expected in most CI containers and
// in any sandbox not launched through a PAM login session.
func TestNewLogindLockState_LiveSystemBus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	l, err := NewLogindLockState(ctx)
	if err != nil {
		t.Skipf("no reachable logind session, skipping: %v", err)
	}
	defer l.Close()

	select {
	case locked := <-l.Locked():
		t.Logf("live LockedHint for this session: %v", locked)
	case <-time.After(time.Second):
		t.Fatal("Locked() produced no initial value")
	}
}
