package input

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/holoplot/go-evdev"
)

// ErrHyprlandUnsupported is returned by NewHyprlandSource when the
// process isn't running under Hyprland (no HYPRLAND_INSTANCE_SIGNATURE)
// or XDG_RUNTIME_DIR is unset, so the compositor's IPC sockets cannot be
// located. Callers should treat it as "this fallback does not apply
// here," never as a startup failure.
var ErrHyprlandUnsupported = errors.New("input: not running under Hyprland")

// hyprlandNames are the two global-shortcut identities registered with
// Hyprland: one per Shift key side. Both the .socket.sock bindn
// registration below and the hyprland_global_shortcuts_manager_v1
// registration in hyprland_globalshortcuts.go key off these same
// strings — Hyprland correlates a physical-key bind to a protocol
// registration by exact id match.
//
// copper-l2z.55's live verification found Hyprland's "global" dispatcher
// forwards the actual current key state to whichever bind fires,
// regardless of the bindn/bindrn flavor of that bind — a single bind
// already observed BOTH the press and the release edge of one physical
// tap on its own shortcut object. copper-l2z.74 found the earlier
// four-identity design (a separate bindn-bound "_down" and
// bindrn-bound "_up" identity per side) therefore registered two
// objects that each independently received both edges, so one tap
// produced two Pressed and two Released events — which resets
// internal/hotkey.Detector's chord state machine on the duplicate press
// and breaks double-Shift-tap detection entirely. One identity per side
// is both sufficient and required.
var hyprlandNames = struct {
	l, r string
}{
	l: "ingot:shiftl",
	r: "ingot:shiftr",
}

// hyprlandBindCommands registers both binds, one per Shift side. "bindn"
// is Hyprland's non-consuming bind flag — confirmed by the epic's live
// verification to leave Shift still reaching the focused application,
// unlike a plain "bind" which would swallow it — and, per the
// hyprlandNames doc above, a single bindn per side already delivers both
// the press and the release edge to its shortcut object, so no
// complementary bindrn is registered.
//
// Two wire details, both established live in copper-l2z.73. The leading
// "= " in "bindn = MODS, ..." is config-file assignment syntax only;
// sent over the raw .socket.sock wire it makes Hyprland reject the whole
// command ("Invalid mod, requested mod \"=\" is not a valid mod"), so it
// must never appear here. And the dispatcher arg is prefixed with
// gsAppID + ":" because Hyprland's global dispatcher splits its argument
// on the FIRST colon into APPID/NAME
// (src/config/shared/actions/ConfigActions.cpp); without that prefix the
// bare hyprlandNames.* id mis-splits and isTaken() against the app_id
// registered in hyprland_globalshortcuts.go silently never matches.
var hyprlandBindCommands = []string{
	"keyword bindn , Shift_L, global, " + gsAppID + ":" + hyprlandNames.l,
	"keyword bindn , Shift_R, global, " + gsAppID + ":" + hyprlandNames.r,
}

// hyprlandUnbindCommands removes every bind hyprlandBindCommands
// registered. "unbind" is keyed by MODS,KEY. hyprctl keyword bindn is not
// idempotent — three identical calls with no intervening unbind produce
// three duplicate binds, live-verified in copper-l2z.50 — so
// registerBinds runs this unconditionally before every bind, not only
// on Pause/Close, on the assumption that Shift_L/Shift_R carry no other
// legitimate bind this process would be clobbering. Same "= " wire
// caveat as hyprlandBindCommands applies here.
var hyprlandUnbindCommands = []string{
	"keyword unbind , Shift_L",
	"keyword unbind , Shift_R",
}

// dialFunc opens a connection to a Hyprland IPC socket. Tests substitute
// one that dials a fake listener instead of a real compositor socket.
type dialFunc func(path string) (net.Conn, error)

func dialUnix(path string) (net.Conn, error) {
	return net.Dial("unix", path)
}

// hyprlandSource is the input.Source that fires the double-Shift chord's
// individual press/release events via Hyprland's native global-keybind
// mechanism instead of reading /dev/input directly — the fallback for
// when evdev device nodes are unreadable. It never consumes the Shift
// key (see hyprlandBindCommands), and internal/hotkey does the
// double-tap timing exactly as it does for the real evdev.Source, since
// Hyprland itself has no double-tap primitive.
//
// Activation delivery is split across two IPC channels for a reason:
// .socket.sock's bindn registers the physical-key-to-id mapping
// compositor-side, but Hyprland's "global" dispatcher only reaches a
// client that separately registered the same id through
// org.freedesktop.portal.GlobalShortcuts or, as used here,
// hyprland_global_shortcuts_manager_v1 — see
// hyprland_globalshortcuts.go for why the portal path doesn't work for
// a plain Go binary and .socket2.sock never fires at all.
type hyprlandSource struct {
	reqPath string
	dial    dialFunc
	wlDial  wlDialFunc
	logger  *slog.Logger

	events chan Event

	mu        sync.Mutex
	shortcuts *globalShortcutsClient
	paused    bool
	closed    bool

	closeOnce sync.Once
}

var _ Source = (*hyprlandSource)(nil)
var _ Pauser = (*hyprlandSource)(nil)

// NewHyprlandSource connects to the running Hyprland compositor's IPC
// socket and Wayland registry, registers the two global binds, and
// starts listening for their activation via
// hyprland_global_shortcuts_manager_v1. It returns
// ErrHyprlandUnsupported without touching anything if Hyprland isn't
// detected.
func NewHyprlandSource() (Source, error) {
	sig := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if sig == "" || runtimeDir == "" {
		return nil, ErrHyprlandUnsupported
	}
	dir := filepath.Join(runtimeDir, "hypr", sig)
	return newHyprlandSource(filepath.Join(dir, ".socket.sock"), dialUnix, dialWaylandSocket, slog.Default())
}

func newHyprlandSource(reqPath string, dial dialFunc, wlDial wlDialFunc, logger *slog.Logger) (*hyprlandSource, error) {
	if logger == nil {
		logger = slog.Default()
	}
	s := &hyprlandSource{
		reqPath: reqPath,
		dial:    dial,
		wlDial:  wlDial,
		logger:  logger,
		events:  make(chan Event, 64),
	}

	if err := s.registerBinds(); err != nil {
		s.unregisterBinds()
		return nil, fmt.Errorf("input: hyprland: register binds: %w", err)
	}

	if err := s.connectShortcuts(); err != nil {
		s.unregisterBinds()
		return nil, fmt.Errorf("input: hyprland: connect global shortcuts: %w", err)
	}

	return s, nil
}

func (s *hyprlandSource) Events() <-chan Event {
	return s.events
}

// Pause unregisters every bind and drops the global-shortcuts
// connection, so no Shift activity reaches this process while paused —
// the same guarantee evdevSource.Pause gives, required while the
// session is locked.
func (s *hyprlandSource) Pause() {
	s.mu.Lock()
	if s.paused || s.closed {
		s.mu.Unlock()
		return
	}
	s.paused = true
	shortcuts := s.shortcuts
	s.shortcuts = nil
	s.mu.Unlock()

	if shortcuts != nil {
		_ = shortcuts.Close()
	}
	s.unregisterBinds()
}

// Resume re-registers every bind and reconnects the global-shortcuts
// client.
func (s *hyprlandSource) Resume() {
	s.mu.Lock()
	if !s.paused || s.closed {
		s.mu.Unlock()
		return
	}
	s.paused = false
	s.mu.Unlock()

	if err := s.registerBinds(); err != nil {
		s.logger.Warn("input: hyprland: resume: register binds", "err", err)
		return
	}
	if err := s.connectShortcuts(); err != nil {
		s.logger.Warn("input: hyprland: resume: connect global shortcuts", "err", err)
	}
}

func (s *hyprlandSource) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		shortcuts := s.shortcuts
		s.shortcuts = nil
		s.mu.Unlock()

		if shortcuts != nil {
			_ = shortcuts.Close()
		}
		s.unregisterBinds()
		close(s.events)
	})
	return nil
}

// sendCommand opens a fresh connection to the request socket, writes one
// command, reads its reply, and closes the connection immediately.
// Hyprland evaluates a request-socket connection synchronously and holds
// up compositor input handling for the duration it stays open; issuing
// one command per connection and closing right after — never reusing a
// connection across commands — is what keeps that window down at
// microseconds instead of freezing the compositor.
func (s *hyprlandSource) sendCommand(cmd string) error {
	conn, err := s.dial(s.reqPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(cmd)); err != nil {
		return err
	}
	// The reply is read to completion (and discarded) before returning,
	// which drains the socket so Hyprland's write side never blocks on
	// this connection being torn down mid-reply.
	buf := make([]byte, 4096)
	for {
		if _, err := conn.Read(buf); err != nil {
			break
		}
	}
	return nil
}

// registerBinds unbinds any stale registration first — hyprctl keyword
// bindn is not idempotent, so skipping this on every call but the first
// would accumulate duplicate binds across repeated Resume calls — then
// issues every bindn command needed for the two global shortcuts,
// stopping at the first failure so a caller sees exactly which one
// didn't take.
func (s *hyprlandSource) registerBinds() error {
	s.unregisterBinds()
	for _, cmd := range hyprlandBindCommands {
		if err := s.sendCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

// unregisterBinds is best-effort: an unbind against a key with nothing
// bound is a harmless no-op, so this runs unconditionally both before
// every bind (see registerBinds) and from Pause/Close, where a failure
// has no good recovery beyond logging it.
func (s *hyprlandSource) unregisterBinds() {
	for _, cmd := range hyprlandUnbindCommands {
		if err := s.sendCommand(cmd); err != nil {
			s.logger.Warn("input: hyprland: unbind", "cmd", cmd, "err", err)
		}
	}
}

func (s *hyprlandSource) connectShortcuts() error {
	ctx, cancel := context.WithTimeout(context.Background(), gsConnectTimeout)
	defer cancel()

	shortcuts, err := dialGlobalShortcuts(ctx, s.wlDial, gsShortcutDefs(), s.emit, s.logger)
	if err != nil {
		return err
	}

	s.mu.Lock()
	if s.closed || s.paused {
		// A Pause or Close raced this dial to completion — the binds
		// this connection depends on may already be gone (Close/Pause
		// unbind under this same lock), and nothing else will ever
		// call Close on an orphaned client. Tear it down instead of
		// storing it.
		s.mu.Unlock()
		_ = shortcuts.Close()
		return nil
	}
	s.shortcuts = shortcuts
	s.mu.Unlock()
	return nil
}

// emit is the globalShortcutsClient's onEvent callback: it reduces a
// (slot, pressed) pair into an Event and delivers it, unless this
// Source has since been paused or closed. Each slot's shortcut object
// receives exactly one Pressed and one Released activation per physical
// tap (see the hyprlandNames doc), so pressed is passed straight through
// from the wire opcode with no de-duplication needed here.
func (s *hyprlandSource) emit(slot gsShortcutSlot, pressed bool) {
	s.mu.Lock()
	closed := s.closed || s.paused
	s.mu.Unlock()
	if closed {
		return
	}

	code, ok := hyprlandSlotKeyCode(slot)
	if !ok {
		return
	}
	ev := Event{Kind: KindShift, Code: code, Pressed: pressed, At: time.Now()}

	select {
	case s.events <- ev:
	default:
		// The channel only backs up if nothing is draining Events() at
		// all; dropping a chord edge here is preferable to blocking the
		// compositor's own event delivery.
	}
}

// hyprlandSlotKeyCode maps a shortcut slot to the evdev key code its
// Shift side corresponds to. Only the side matters here — press vs.
// release is carried by the pressed bool emit's caller already received
// from the wire opcode.
func hyprlandSlotKeyCode(slot gsShortcutSlot) (uint16, bool) {
	switch slot {
	case gsSlotL:
		return uint16(evdev.KEY_LEFTSHIFT), true
	case gsSlotR:
		return uint16(evdev.KEY_RIGHTSHIFT), true
	default:
		return 0, false
	}
}
