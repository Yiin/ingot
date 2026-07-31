package session

import (
	"context"
	"log/slog"

	"github.com/Yiin/ingot/internal/input"
)

// Gate drives an input.Pauser from a LockState, so no evdev reads happen
// while the session is locked. It takes a Pauser rather than a plain
// input.Source deliberately: filtering already-read events downstream
// would leave the keyboard devices open and being read while locked,
// which is exactly the exposure this package exists to close.
type Gate struct {
	src    input.Pauser
	lock   LockState
	logger *slog.Logger
}

// NewGate builds a Gate over src, driven by lock. Call Run to start
// tracking; it blocks until ctx is done or lock's channel is closed.
func NewGate(src input.Pauser, lock LockState, logger *slog.Logger) *Gate {
	if logger == nil {
		logger = slog.Default()
	}
	return &Gate{src: src, lock: lock, logger: logger}
}

// Run applies every lock-state update from g.lock to g.src until ctx is
// done. The first value it observes sets the initial state — including
// pausing immediately if the session is already locked when Run starts —
// so Run should be started before any input events are expected to flow.
func (g *Gate) Run(ctx context.Context) {
	locked := false
	for {
		select {
		case <-ctx.Done():
			return
		case v, ok := <-g.lock.Locked():
			if !ok {
				return
			}
			if v == locked {
				continue
			}
			locked = v
			if locked {
				g.logger.Info("session: locked, pausing input capture")
				g.src.Pause()
			} else {
				g.logger.Info("session: unlocked, resuming input capture")
				g.src.Resume()
			}
		}
	}
}
