// Package selection reads the Wayland PRIMARY selection and CLIPBOARD by
// shelling out to wl-paste.
//
// A native protocol client was considered and rejected: as of writing,
// github.com/rajveermalviya/go-wayland has been stale since 2023-01-30 and
// ships no data-control bindings for either ext-data-control-v1 or
// wlr-data-control-v1. Generating and hand-maintaining a protocol client
// would save well under two milliseconds — wl-paste --primary --no-newline
// measured 1.8 ms average over 20 runs on the dev machine.
//
// PRIMARY carries no freshness signal. wl-paste returns the last selection
// made anywhere in the session, not necessarily the one the user just
// made — during testing it returned a sentence selected minutes earlier in
// an unrelated window. macOS Accessibility gives the current selection;
// this is weaker. Callers should show the captured text to the user before
// acting on it and keep the capture undoable.
//
// Self-capture: callers should read PRIMARY before presenting the panel,
// and ignore PRIMARY while the panel owns keyboard focus — Ingot's own
// text entry will claim PRIMARY the moment the user selects text inside
// it, which would otherwise make the panel capture its own contents.
package selection
