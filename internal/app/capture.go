package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/Yiin/ingot/internal/input"
	"github.com/Yiin/ingot/internal/selection"
	"github.com/Yiin/ingot/internal/session"
	"github.com/Yiin/ingot/internal/setup"
	"github.com/Yiin/ingot/internal/store"
)

// primaryReadTimeout bounds the PRIMARY-selection worker started by
// onChordFired — generous over selection.WlPasteReader's own 2s
// internal timeout so that timeout, not this one, is what actually
// fires on a stuck wl-paste.
const primaryReadTimeout = 3 * time.Second

// chordProbeReason is chordDecision's pure classification of a keyboard
// probe into the reason startChord should disable the chord, or "" if
// the probe found a usable keyboard. Split out from startChord so the
// decision is testable with plain go test — everything downstream of it
// (opening evdev, wiring the gate) needs a real machine and cannot run
// here anyway.
func chordProbeReason(status setup.KeyboardStatus, probeErr error) string {
	switch {
	case probeErr != nil:
		return "Could not check for a keyboard device (" + probeErr.Error() + ")."
	case status.Detected == 0:
		return "No keyboard-capable input devices were detected. The global Shift-Shift chord is off."
	case status.Readable == 0:
		return "Detected a keyboard but can't read it — the global Shift-Shift chord is off. Run `ingot setup`."
	default:
		return ""
	}
}

// startChord probes for a readable keyboard and, if one exists, starts
// the evdev reader, the lock-state gate, and the double-Shift-tap
// detector. If none is readable, the app still runs — see the child's
// "degradation is a hard requirement" acceptance criterion — with a
// persistent notice on the panel explaining why and pointing at `ingot
// setup`.
func (a *App) startChord() {
	status, err := setup.ProbeKeyboards("", "")
	if reason := chordProbeReason(status, err); reason != "" {
		a.setChordDisabled(reason)
		return
	}

	src, err := input.NewSource()
	if err != nil {
		a.setChordDisabled("Could not open input devices (" + err.Error() + "). The global Shift-Shift chord is off.")
		return
	}
	a.src = src
	a.reader = selection.NewReader()
	a.shell.SetNotice("")

	if status.Readable < status.Detected {
		slog.Warn("app: some keyboards are not readable; the chord may miss events from them")
	}

	if lock, err := session.NewLogindLockState(context.Background()); err != nil {
		slog.Warn("app: session lock-state unavailable; the chord will keep running through screen locks", "err", err)
	} else {
		a.lock = lock
		gateCtx, cancel := context.WithCancel(context.Background())
		a.gateCancel = cancel
		gate := session.NewGate(src.(input.Pauser), lock, slog.Default())
		goSafe("session-gate", func() { gate.Run(gateCtx) })
	}

	goSafe("evdev-reader", a.readInput)
}

// setChordDisabled records that the global chord is off and surfaces it
// on the panel as a standing notice — never fatal, per the epic's
// degradation rule that a missing permission degrades one feature, not
// the app.
func (a *App) setChordDisabled(reason string) {
	slog.Warn("app: " + reason)
	a.shell.SetNotice(reason)
}

// readInput drains the evdev source, feeding every event to the
// double-Shift-tap detector and posting onChordFired to the GTK thread
// whenever it fires. Runs until src.Events() closes (Source.Close).
func (a *App) readInput() {
	for ev := range a.src.Events() {
		if a.detector.Feed(ev) {
			a.gapp.Post(a.onChordFired)
		}
	}
}

// onChordFired runs on the GTK thread (posted from readInput). It reads
// PRIMARY on a worker goroutine — selection.WlPasteReader shells out and
// can block for up to 2s, which would freeze the panel if done inline —
// and posts the result back.
func (a *App) onChordFired() {
	goSafe("primary-read", func() {
		ctx, cancel := context.WithTimeout(context.Background(), primaryReadTimeout)
		defer cancel()
		text, err := a.reader.Primary(ctx)
		a.gapp.Post(func() { a.handleCapture(text, err) })
	})
}

// handleCapture is onChordFired's continuation back on the GTK thread:
// classify the read, then either show "nothing selected", flash an
// existing duplicate row, or append a new captured note and show it.
// Never presents the panel — capture must not steal focus permanently,
// so the only visible response is the HUD toast and (if the panel
// happens to already be visible) scrolling the new row into view.
func (a *App) handleCapture(text string, err error) {
	if errors.Is(err, selection.ErrEmpty) {
		a.shell.NotifyEmptySelection()
		return
	}
	if err != nil {
		slog.Warn("app: capture: read PRIMARY", "err", err)
		a.notifier().Captured("Capture failed")
		return
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		a.shell.NotifyEmptySelection()
		return
	}

	if dup := a.lastCapturedNote(); dup != nil && strings.TrimSpace(dup.Body) == trimmed {
		a.shell.NotifyDuplicate(a.adapter.itemForNote(dup.ID))
		return
	}

	id, err := a.store.AppendToDefault(text, store.Origin{})
	if err != nil {
		slog.Warn("app: capture: append note", "err", err)
		a.notifier().Captured("Capture failed")
		return
	}

	a.notifier().Captured("Captured: " + text)
	a.shell.RefreshEmptyState("", 0)

	if a.visible {
		if it := a.adapter.itemForNote(id); it != nil {
			a.shell.List().ScrollTo(it)
		}
	}
}

// lastCapturedNote returns the most recent note in the active project's
// default capture location (its last section), for the capture flow's
// duplicate check, or nil if that section has no notes yet.
func (a *App) lastCapturedNote() *store.Note {
	proj, err := a.store.Project(a.store.Active())
	if err != nil || len(proj.Sections) == 0 {
		return nil
	}
	sec := proj.Sections[len(proj.Sections)-1]
	if len(sec.Notes) == 0 {
		return nil
	}
	n := sec.Notes[len(sec.Notes)-1]
	return &n
}
