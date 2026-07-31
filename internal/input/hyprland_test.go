package input

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeReqServer stands in for Hyprland's .socket.sock: it accepts a
// fresh connection per command, records the bytes written to it, replies
// with a fixed "ok" payload, and closes — mirroring sendCommand's own
// one-shot-per-command discipline from the other side.
type fakeReqServer struct {
	ln net.Listener

	mu   sync.Mutex
	cmds []string
}

func newFakeReqServer(t *testing.T, path string) *fakeReqServer {
	t.Helper()
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen %s: %v", path, err)
	}
	s := &fakeReqServer{ln: ln}
	go s.serve()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *fakeReqServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			buf := make([]byte, 4096)
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			s.mu.Lock()
			s.cmds = append(s.cmds, string(buf[:n]))
			s.mu.Unlock()
			_, _ = conn.Write([]byte("ok"))
		}()
	}
}

func (s *fakeReqServer) commands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.cmds...)
}

// fakeGlobalShortcutsServer stands in for the Wayland compositor socket
// this package binds hyprland_global_shortcuts_manager_v1 over. Every
// accepted connection is served the get_registry/sync/bind/
// register_shortcut handshake before being stored, so a test can push
// pressed/released events into it afterward. One connection is
// tracked at a time, mirroring what Pause/Resume actually do to the real
// connection: drop it, then dial fresh.
type fakeGlobalShortcutsServer struct {
	ln net.Listener

	mu   sync.Mutex
	conn net.Conn
}

func newFakeGlobalShortcutsServer(t *testing.T, path string) *fakeGlobalShortcutsServer {
	t.Helper()
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen %s: %v", path, err)
	}
	s := &fakeGlobalShortcutsServer{ln: ln}
	go s.serve()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *fakeGlobalShortcutsServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go func() {
			if err := s.handshake(conn); err != nil {
				conn.Close()
				return
			}
			s.mu.Lock()
			s.conn = conn
			s.mu.Unlock()
		}()
	}
}

// handshake replays exactly what dialGlobalShortcuts expects: a single
// wl_registry.global advertising hyprland_global_shortcuts_manager_v1 at
// version 1, the sync callback's done event, then the bind request and
// two register_shortcut requests read and discarded.
func (s *fakeGlobalShortcutsServer) handshake(conn net.Conn) error {
	var discard [24]byte // get_registry (12 bytes) + sync (12 bytes)
	if _, err := io.ReadFull(conn, discard[:]); err != nil {
		return err
	}

	global := &gsEncoder{}
	global.uint32(99) // registry name
	global.string(hyprlandGlobalShortcutsInterface)
	global.uint32(1) // version
	if err := writeGsMessage(conn, gsRegistryID, gsRegistryEventGlobal, global.buf); err != nil {
		return err
	}
	done := &gsEncoder{}
	done.uint32(0)
	if err := writeGsMessage(conn, gsCallbackID, gsCallbackEventDone, done.buf); err != nil {
		return err
	}

	for i := 0; i < 3; i++ { // bind + 2x register_shortcut
		if _, _, err := readGsMessage(conn); err != nil {
			return err
		}
	}
	return nil
}

func (s *fakeGlobalShortcutsServer) dial(ctx context.Context) (net.Conn, error) {
	return net.Dial("unix", s.ln.Addr().String())
}

// sendShortcut waits briefly for a connection to complete its handshake,
// then writes one pressed/released event for slot to it. Tests
// call this only after an action (construction or Resume) that is
// expected to reconnect.
func (s *fakeGlobalShortcutsServer) sendShortcut(t *testing.T, slot gsShortcutSlot, pressed bool) {
	t.Helper()
	var objectID uint32
	for _, def := range gsShortcutDefs() {
		if def.slot == slot {
			objectID = def.objectID
			break
		}
	}
	opcode := gsShortcutEventReleased
	if pressed {
		opcode = gsShortcutEventPressed
	}

	body := &gsEncoder{}
	body.uint32(0) // tv_sec_hi
	body.uint32(0) // tv_sec_lo
	body.uint32(0) // tv_nsec

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		conn := s.conn
		s.mu.Unlock()
		if conn != nil {
			if err := writeGsMessage(conn, objectID, opcode, body.buf); err == nil {
				return
			}
			// After Pause/Resume the source drops its connection and
			// dials a fresh one. s.conn still points at the closed
			// pre-Pause conn until the new handshake completes, so a
			// write here fails with EPIPE. Forget it and wait for the
			// reconnect rather than failing the test on the race.
			s.mu.Lock()
			if s.conn == conn {
				s.conn = nil
			}
			s.mu.Unlock()
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("no live global-shortcuts connection accepted before deadline")
}

// sendUnrelated writes one event for an object id this package never
// registers, so a test can prove it is skipped rather than misread as a
// shortcut activation or corrupting the message stream's framing.
func (s *fakeGlobalShortcutsServer) sendUnrelated(t *testing.T) {
	t.Helper()
	body := &gsEncoder{}
	body.uint32(1)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		conn := s.conn
		s.mu.Unlock()
		if conn != nil {
			if err := writeGsMessage(conn, 999, 0, body.buf); err != nil {
				t.Fatalf("write unrelated event: %v", err)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("no global-shortcuts connection accepted before deadline")
}

func testSockets(t *testing.T) (reqPath, wlPath string) {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "req.sock"), filepath.Join(dir, "wl.sock")
}

func TestNewHyprlandSourceUnsupportedWithoutEnv(t *testing.T) {
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	if _, err := NewHyprlandSource(); err != ErrHyprlandUnsupported {
		t.Fatalf("NewHyprlandSource() err = %v, want ErrHyprlandUnsupported", err)
	}
}

func TestNewHyprlandSourceResolvesSocketPathsFromEnv(t *testing.T) {
	dir := t.TempDir()
	sig := "sig123"
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", sig)
	t.Setenv("XDG_RUNTIME_DIR", dir)

	hyprDir := filepath.Join(dir, "hypr", sig)
	if err := os.MkdirAll(hyprDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	newFakeReqServer(t, filepath.Join(hyprDir, ".socket.sock"))
	gs := newFakeGlobalShortcutsServer(t, filepath.Join(dir, "wayland-test.sock"))
	t.Setenv("WAYLAND_DISPLAY", filepath.Join(dir, "wayland-test.sock"))

	src, err := NewHyprlandSource()
	if err != nil {
		t.Fatalf("NewHyprlandSource: %v", err)
	}
	defer src.Close()
	_ = gs
}

func TestHyprlandSourceRegistersBothBinds(t *testing.T) {
	reqPath, wlPath := testSockets(t)
	req := newFakeReqServer(t, reqPath)
	gs := newFakeGlobalShortcutsServer(t, wlPath)

	src, err := newHyprlandSource(reqPath, dialUnix, gs.dial, nil)
	if err != nil {
		t.Fatalf("newHyprlandSource: %v", err)
	}
	defer src.Close()

	// registerBinds unconditionally unbinds before binding (the fix for
	// hyprctl keyword bindn's non-idempotence), so construction sends
	// the two unbind commands first, then the two bind commands.
	want := len(hyprlandUnbindCommands) + len(hyprlandBindCommands)
	waitForCount(t, func() int { return len(req.commands()) }, want)
	got := req.commands()

	for i, wantCmd := range hyprlandUnbindCommands {
		if got[i] != wantCmd {
			t.Errorf("command[%d] = %q, want %q", i, got[i], wantCmd)
		}
	}
	for i, wantCmd := range hyprlandBindCommands {
		idx := len(hyprlandUnbindCommands) + i
		if got[idx] != wantCmd {
			t.Errorf("command[%d] = %q, want %q", idx, got[idx], wantCmd)
		}
	}
}

func TestHyprlandSourceEmitsPressAndReleaseEvents(t *testing.T) {
	reqPath, wlPath := testSockets(t)
	newFakeReqServer(t, reqPath)
	gs := newFakeGlobalShortcutsServer(t, wlPath)

	src, err := newHyprlandSource(reqPath, dialUnix, gs.dial, nil)
	if err != nil {
		t.Fatalf("newHyprlandSource: %v", err)
	}
	defer src.Close()

	// Both edges of one physical tap arrive on the SAME slot's object —
	// see the hyprlandNames doc in hyprland.go for why a tap is no
	// longer split across a separate "_down"/"_up" object pair.
	gs.sendShortcut(t, gsSlotL, true)
	ev := mustRecvEvent(t, src)
	if ev.Kind != KindShift || !ev.Pressed {
		t.Errorf("got %+v, want a Shift press", ev)
	}

	gs.sendShortcut(t, gsSlotL, false)
	ev = mustRecvEvent(t, src)
	if ev.Kind != KindShift || ev.Pressed {
		t.Errorf("got %+v, want a Shift release", ev)
	}
}

// TestGsShortcutDefsOneObjectPerSide pins down the actual shape of
// copper-l2z.74's fix at the source: exactly one registered shortcut
// object per Shift side, bound by exactly one command. Without this
// check, a regression back to a separate "_down"/"_up" object pair per
// side would still compile and still pass a fake-driven "one event per
// send" test like the one below, since such a fake only proves the
// client doesn't duplicate an event it wasn't sent — the original bug
// was Hyprland fanning both edges out to every registered object, which
// this test catches at the registration count instead.
func TestGsShortcutDefsOneObjectPerSide(t *testing.T) {
	defs := gsShortcutDefs()
	if len(defs) != 2 {
		t.Fatalf("gsShortcutDefs() has %d entries, want exactly 2 (one per Shift side)", len(defs))
	}
	if len(hyprlandBindCommands) != 2 {
		t.Fatalf("hyprlandBindCommands has %d entries, want exactly 2 (one bindn per Shift side)", len(hyprlandBindCommands))
	}

	seenSlots := map[gsShortcutSlot]bool{}
	for _, def := range defs {
		if seenSlots[def.slot] {
			t.Fatalf("slot %v registered by more than one gsShortcutDef", def.slot)
		}
		seenSlots[def.slot] = true
	}
	if !seenSlots[gsSlotL] || !seenSlots[gsSlotR] {
		t.Fatalf("gsShortcutDefs() = %+v, want exactly one gsSlotL and one gsSlotR entry", defs)
	}
}

// TestHyprlandBindCommandWireSyntax guards copper-l2z.73's two live
// findings, which are easy to reintroduce when editing the bind table by
// hand. "= " is config-file assignment syntax; over the raw socket it
// makes Hyprland reject the whole command. And the global dispatcher
// splits its argument on the FIRST colon into APPID/NAME, so a bare
// shortcut id without the gsAppID prefix mis-splits and never matches
// the registered app_id.
func TestHyprlandBindCommandWireSyntax(t *testing.T) {
	for _, cmd := range append(append([]string{}, hyprlandBindCommands...), hyprlandUnbindCommands...) {
		if strings.Contains(cmd, "= ") {
			t.Errorf("bind command %q contains config-file assignment syntax %q; Hyprland rejects it over the socket", cmd, "= ")
		}
	}
	for _, cmd := range hyprlandBindCommands {
		if !strings.Contains(cmd, "global, "+gsAppID+":") {
			t.Errorf("bind command %q does not prefix its dispatcher arg with %q; the global dispatcher would mis-split it", cmd, gsAppID+":")
		}
	}
}

// TestHyprlandSourceTapProducesExactlyOnePressAndOneRelease is
// copper-l2z.74's core regression test: before the fix, a physical tap
// produced two Pressed and two Released events (one from each of a
// bindn-bound and a bindrn-bound object), which reset
// internal/hotkey.Detector's chord state machine on the duplicate press
// and made double-Shift-tap detection non-functional. One shortcut
// object per side must deliver exactly one of each per tap.
func TestHyprlandSourceTapProducesExactlyOnePressAndOneRelease(t *testing.T) {
	reqPath, wlPath := testSockets(t)
	newFakeReqServer(t, reqPath)
	gs := newFakeGlobalShortcutsServer(t, wlPath)

	src, err := newHyprlandSource(reqPath, dialUnix, gs.dial, nil)
	if err != nil {
		t.Fatalf("newHyprlandSource: %v", err)
	}
	defer src.Close()

	gs.sendShortcut(t, gsSlotL, true)
	press := mustRecvEvent(t, src)
	if press.Kind != KindShift || !press.Pressed {
		t.Fatalf("got %+v, want a single Shift press", press)
	}

	gs.sendShortcut(t, gsSlotL, false)
	release := mustRecvEvent(t, src)
	if release.Kind != KindShift || release.Pressed {
		t.Fatalf("got %+v, want a single Shift release", release)
	}

	select {
	case ev, ok := <-src.Events():
		if ok {
			t.Fatalf("got unexpected extra event %+v after one press+release tap", ev)
		}
	case <-time.After(100 * time.Millisecond):
		// No extra event queued, as expected.
	}
}

func TestHyprlandSourceIgnoresUnrelatedEvents(t *testing.T) {
	reqPath, wlPath := testSockets(t)
	newFakeReqServer(t, reqPath)
	gs := newFakeGlobalShortcutsServer(t, wlPath)

	src, err := newHyprlandSource(reqPath, dialUnix, gs.dial, nil)
	if err != nil {
		t.Fatalf("newHyprlandSource: %v", err)
	}
	defer src.Close()

	gs.sendUnrelated(t)
	gs.sendShortcut(t, gsSlotR, true)

	ev := mustRecvEvent(t, src)
	if ev.Kind != KindShift || !ev.Pressed || ev.Code == 0 {
		t.Errorf("got %+v, want the right-Shift press, unrelated event skipped", ev)
	}
}

func TestHyprlandSourceCloseUnregistersBinds(t *testing.T) {
	reqPath, wlPath := testSockets(t)
	req := newFakeReqServer(t, reqPath)
	gs := newFakeGlobalShortcutsServer(t, wlPath)

	src, err := newHyprlandSource(reqPath, dialUnix, gs.dial, nil)
	if err != nil {
		t.Fatalf("newHyprlandSource: %v", err)
	}
	afterConstruct := len(hyprlandUnbindCommands) + len(hyprlandBindCommands)
	waitForCount(t, func() int { return len(req.commands()) }, afterConstruct)

	if err := src.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	afterClose := afterConstruct + len(hyprlandUnbindCommands)
	waitForCount(t, func() int { return len(req.commands()) }, afterClose)

	got := req.commands()[afterConstruct:afterClose]
	for i, want := range hyprlandUnbindCommands {
		if got[i] != want {
			t.Errorf("unbind command[%d] = %q, want %q", i, got[i], want)
		}
	}

	if _, ok := <-src.Events(); ok {
		t.Errorf("Events() channel still open after Close")
	}
}

func TestHyprlandSourcePauseStopsEventsResumeRestartsThem(t *testing.T) {
	reqPath, wlPath := testSockets(t)
	req := newFakeReqServer(t, reqPath)
	gs := newFakeGlobalShortcutsServer(t, wlPath)

	hs, err := newHyprlandSource(reqPath, dialUnix, gs.dial, nil)
	if err != nil {
		t.Fatalf("newHyprlandSource: %v", err)
	}
	defer hs.Close()
	afterConstruct := len(hyprlandUnbindCommands) + len(hyprlandBindCommands)
	waitForCount(t, func() int { return len(req.commands()) }, afterConstruct)

	hs.Pause()
	afterPause := afterConstruct + len(hyprlandUnbindCommands)
	waitForCount(t, func() int { return len(req.commands()) }, afterPause)

	hs.Resume()
	afterResume := afterPause + len(hyprlandUnbindCommands) + len(hyprlandBindCommands)
	waitForCount(t, func() int { return len(req.commands()) }, afterResume)

	gs.sendShortcut(t, gsSlotL, true)
	ev := mustRecvEvent(t, hs)
	if ev.Kind != KindShift || !ev.Pressed {
		t.Errorf("got %+v after Resume, want a Shift press", ev)
	}
}

func waitForCount(t *testing.T, count func() int, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if count() >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("count = %d, want >= %d before deadline", count(), want)
}

func mustRecvEvent(t *testing.T, src Source) Event {
	t.Helper()
	select {
	case ev, ok := <-src.Events():
		if !ok {
			t.Fatalf("Events() channel closed unexpectedly")
		}
		return ev
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for an event")
	}
	return Event{}
}
