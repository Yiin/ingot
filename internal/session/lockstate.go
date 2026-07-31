package session

// LockState reports the desktop session's lock state as a stream of
// updates: true while locked, false while unlocked. The channel holds at
// most the most recent value a consumer hasn't read yet — a slow or
// absent reader is never blocked, and never falls behind on stale
// transitions, only ever sees the latest state.
type LockState interface {
	Locked() <-chan bool
}

// FakeLockState is a LockState double for tests: SetLocked pushes a new
// state without D-Bus or a real logind session.
type FakeLockState struct {
	ch chan bool
}

// NewFakeLockState returns a FakeLockState whose first read is locked.
func NewFakeLockState(locked bool) *FakeLockState {
	f := &FakeLockState{ch: make(chan bool, 1)}
	f.ch <- locked
	return f
}

func (f *FakeLockState) Locked() <-chan bool { return f.ch }

// SetLocked pushes a new state, replacing any unread previous value so
// the channel never blocks the caller and never queues stale states.
func (f *FakeLockState) SetLocked(locked bool) {
	select {
	case <-f.ch:
	default:
	}
	f.ch <- locked
}

var _ LockState = (*FakeLockState)(nil)
