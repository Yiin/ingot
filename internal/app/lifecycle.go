package app

import (
	"context"
	"log/slog"
	"time"
)

// shutdownFlushTimeout bounds the final flush shutdown performs before
// closing the store.
const shutdownFlushTimeout = 2 * time.Second

// shutdown runs Ingot's ordered teardown exactly once, regardless of
// which of its callers reaches it first (SIGTERM/SIGINT, the "quit"
// action, or GtkApplication's own "shutdown" signal firing after Quit
// asks it to): stop watching the colour scheme, stop driving the lock
// gate, stop the store subscription,
// close the input source, flush and close the store, then ask
// GtkApplication to quit — which is itself what fires "shutdown" for
// any caller that arrived some other way.
func (a *App) shutdown() {
	a.shutdownOnce.Do(func() {
		// First: its goroutine posts onto the GTK thread, and Quit below
		// stops the main loop that would run those posts.
		if a.schemeStop != nil {
			a.schemeStop()
		}
		if a.gateCancel != nil {
			a.gateCancel()
		}
		if a.unsub != nil {
			a.unsub()
		}
		if a.src != nil {
			if err := a.src.Close(); err != nil {
				slog.Warn("app: shutdown: close input source", "err", err)
			}
		}
		if a.store != nil {
			ctx, cancel := context.WithTimeout(context.Background(), shutdownFlushTimeout)
			if err := a.store.Flush(ctx); err != nil {
				slog.Warn("app: shutdown: flush", "err", err)
			}
			cancel()
			if err := a.store.Close(); err != nil {
				slog.Warn("app: shutdown: close store", "err", err)
			}
		}
		if a.stopSignals != nil {
			a.stopSignals()
		}
		// Drop the keep-alive before Quit, or the hold taken in startup
		// keeps the main loop running after the quit request.
		if a.releaseHold != nil {
			a.releaseHold()
		}
		if a.gapp != nil {
			a.gapp.Quit()
		}
	})
}
