package session

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

// fakePauser is an input.Pauser double that models "a device is being
// read" as a counter incremented by a background goroutine whenever it
// isn't paused — the same shape as evdevSource's real read loop, minus
// the actual device — so a test can assert the counter is frozen while
// locked and moving again after unlock.
type fakePauser struct {
	stop chan struct{}
	wg   sync.WaitGroup

	mu     sync.Mutex
	paused bool
	reads  int
}

func newFakePauser() *fakePauser {
	f := &fakePauser{stop: make(chan struct{})}
	f.wg.Add(1)
	go f.simulateReads()
	return f
}

func (f *fakePauser) simulateReads() {
	defer f.wg.Done()
	t := time.NewTicker(time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-f.stop:
			return
		case <-t.C:
			f.mu.Lock()
			if !f.paused {
				f.reads++
			}
			f.mu.Unlock()
		}
	}
}

func (f *fakePauser) Pause() {
	f.mu.Lock()
	f.paused = true
	f.mu.Unlock()
}

func (f *fakePauser) Resume() {
	f.mu.Lock()
	f.paused = false
	f.mu.Unlock()
}

func (f *fakePauser) isPaused() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.paused
}

func (f *fakePauser) readCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads
}

func (f *fakePauser) close() {
	close(f.stop)
	f.wg.Wait()
}

func TestGate_PausesWhileLocked_ResumesOnUnlock(t *testing.T) {
	pauser := newFakePauser()
	defer pauser.close()

	lock := NewFakeLockState(false)
	gate := NewGate(pauser, lock, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go gate.Run(ctx)

	// Reads happen while unlocked.
	waitUntil(t, time.Second, func() bool { return pauser.readCount() > 0 })

	lock.SetLocked(true)
	waitUntil(t, time.Second, pauser.isPaused)

	frozen := pauser.readCount()
	time.Sleep(50 * time.Millisecond)
	if got := pauser.readCount(); got != frozen {
		t.Errorf("read count advanced from %d to %d while locked", frozen, got)
	}

	lock.SetLocked(false)
	waitUntil(t, time.Second, func() bool { return !pauser.isPaused() })
	waitUntil(t, time.Second, func() bool { return pauser.readCount() > frozen })
}

func TestGate_AlreadyLockedAtStart_PausesImmediately(t *testing.T) {
	pauser := newFakePauser()
	defer pauser.close()

	lock := NewFakeLockState(true)
	gate := NewGate(pauser, lock, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go gate.Run(ctx)

	waitUntil(t, time.Second, pauser.isPaused)
}

func TestGate_StopsOnContextCancel(t *testing.T) {
	pauser := newFakePauser()
	defer pauser.close()

	lock := NewFakeLockState(false)
	gate := NewGate(pauser, lock, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		gate.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after ctx was canceled")
	}
}
