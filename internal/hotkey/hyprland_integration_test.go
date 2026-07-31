package hotkey_test

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/hotkey"
	"github.com/Yiin/ingot/internal/input"
)

// This file drives a real internal/hotkey.Detector off
// internal/input.NewHyprlandSource, replaying just enough of Hyprland's
// two IPC channels (the .socket.sock request socket and the Wayland
// hyprland_global_shortcuts_manager_v1 protocol) to stand in for a live
// compositor. It is copper-l2z.74's acceptance test: before that fix, a
// physical Shift tap fired two Pressed and two Released events (one
// object bound via bindn, one via bindrn, both receiving both edges),
// and Detector.Feed resets its whole chord state machine on the second,
// duplicate Pressed — making a real double-Shift-tap chord
// undetectable via this fallback. With one shortcut object per side,
// exactly one Pressed and one Released event reach Detector per tap, so
// a double-tap fires correctly.
//
// This lives in package hotkey_test (black-box), not internal/input's
// own test suite, because internal/hotkey already imports internal/input
// one-directionally — internal/input importing internal/hotkey back
// would be an import cycle. It can only reach internal/input's exported
// API, so it hand-rolls the small subset of the Wayland wire protocol
// needed to fake a compositor, duplicating the object ids
// internal/input's client hard-codes for its own fixed setup sequence
// (wl_display=1, wl_registry=2, sync callback=3,
// hyprland_global_shortcuts_manager_v1=4 — see hyprland_globalshortcuts.go).

const (
	wlDisplayID  uint32 = 1
	wlRegistryID uint32 = 2
	wlCallbackID uint32 = 3
	wlManagerID  uint32 = 4

	wlGlobalShortcutsInterface = "hyprland_global_shortcuts_manager_v1"

	wlShortcutEventPressed  uint16 = 0
	wlShortcutEventReleased uint16 = 1
)

func writeWlMessage(w io.Writer, objectID uint32, opcode uint16, body []byte) error {
	size := 8 + len(body)
	buf := make([]byte, 8, size)
	binary.LittleEndian.PutUint32(buf[0:4], objectID)
	binary.LittleEndian.PutUint16(buf[4:6], opcode)
	binary.LittleEndian.PutUint16(buf[6:8], uint16(size))
	buf = append(buf, body...)
	_, err := w.Write(buf)
	return err
}

func wlUint32(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

func wlString(s string) []byte {
	b := wlUint32(uint32(len(s) + 1))
	b = append(b, s...)
	b = append(b, 0)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

type wlHeader struct {
	objectID uint32
	opcode   uint16
	size     uint16
}

func readWlMessage(r io.Reader) (wlHeader, []byte, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return wlHeader{}, nil, err
	}
	h := wlHeader{
		objectID: binary.LittleEndian.Uint32(hdr[0:4]),
		opcode:   binary.LittleEndian.Uint16(hdr[4:6]),
		size:     binary.LittleEndian.Uint16(hdr[6:8]),
	}
	payload := make([]byte, int(h.size)-8)
	if len(payload) > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return wlHeader{}, nil, err
		}
	}
	return h, payload, nil
}

// fakeReqServer stands in for Hyprland's .socket.sock: it accepts a bind
// command connection, replies, and closes. The command content is
// irrelevant to this test — only that sendCommand's write-then-drain
// roundtrip completes without hanging.
type fakeReqServer struct{ ln net.Listener }

func newFakeReqServer(t *testing.T, path string) *fakeReqServer {
	t.Helper()
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen %s: %v", path, err)
	}
	s := &fakeReqServer{ln: ln}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				buf := make([]byte, 4096)
				if _, err := conn.Read(buf); err == nil {
					_, _ = conn.Write([]byte("ok"))
				}
			}()
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

// fakeCompositor stands in for the Wayland socket
// internal/input.NewHyprlandSource dials to reach
// hyprland_global_shortcuts_manager_v1. It replays the fixed setup
// handshake internal/input's client performs, captures the object id(s)
// the client assigns via register_shortcut, and lets the test push
// pressed/released events for a captured id afterward.
type fakeCompositor struct {
	ln net.Listener

	mu           sync.Mutex
	conn         net.Conn
	shortcutIDs  []uint32
	handshakeErr error
}

func newFakeCompositor(t *testing.T, path string) *fakeCompositor {
	t.Helper()
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen %s: %v", path, err)
	}
	c := &fakeCompositor{ln: ln}
	go c.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return c
}

func (c *fakeCompositor) serve() {
	conn, err := c.ln.Accept()
	if err != nil {
		return
	}
	if err := c.handshake(conn); err != nil {
		c.mu.Lock()
		c.handshakeErr = err
		c.mu.Unlock()
		_ = conn.Close()
		return
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
}

func (c *fakeCompositor) handshake(conn net.Conn) error {
	// get_registry (12 bytes) + sync (12 bytes): fixed-shape requests
	// whose only argument is a single uint32, discarded unread.
	var discard [24]byte
	if _, err := io.ReadFull(conn, discard[:]); err != nil {
		return err
	}

	global := wlUint32(99) // registry name
	global = append(global, wlString(wlGlobalShortcutsInterface)...)
	global = append(global, wlUint32(1)...) // version
	if err := writeWlMessage(conn, wlRegistryID, 0, global); err != nil {
		return err
	}
	if err := writeWlMessage(conn, wlCallbackID, 0, wlUint32(0)); err != nil {
		return err
	}

	// bind + one register_shortcut per Shift side. The client registers
	// two shortcuts (copper-l2z.74 collapsed this from four); capture the
	// object id of each register_shortcut request (objectID == wlManagerID)
	// in registration order so the test can drive events against them
	// without needing to know internal/input's own id/name strings.
	for i := 0; i < 3; i++ {
		h, payload, err := readWlMessage(conn)
		if err != nil {
			return err
		}
		if h.objectID == wlManagerID && len(payload) >= 4 {
			c.mu.Lock()
			c.shortcutIDs = append(c.shortcutIDs, binary.LittleEndian.Uint32(payload[:4]))
			c.mu.Unlock()
		}
	}
	return nil
}

// firstShortcutID waits for the handshake to capture BOTH registered
// shortcut object ids — asserting exactly two, not merely "at least
// one", is what makes this test actually pin down copper-l2z.74's fix
// (one object per Shift side, not one per press/release edge): a
// regression back to four objects would still compile and still
// deliver one event per send here, so without this count check the test
// would silently stop testing the thing it exists to test. It returns
// the first id, which this test drives both edges of a tap through.
func (c *fakeCompositor) firstShortcutID(t *testing.T) uint32 {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		err := c.handshakeErr
		ids := append([]uint32(nil), c.shortcutIDs...)
		c.mu.Unlock()
		if err != nil {
			t.Fatalf("compositor handshake: %v", err)
		}
		if len(ids) >= 2 {
			if len(ids) != 2 {
				t.Fatalf("registered %d global shortcuts, want exactly 2 (one per Shift side)", len(ids))
			}
			return ids[0]
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("did not observe 2 registered shortcuts before deadline")
	return 0
}

// connection waits for the handshake to finish and the compositor
// connection to be stored, so sendTap never races handshake's last
// pending read.
func (c *fakeCompositor) connection(t *testing.T) net.Conn {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		conn := c.conn
		err := c.handshakeErr
		c.mu.Unlock()
		if err != nil {
			t.Fatalf("compositor handshake: %v", err)
		}
		if conn != nil {
			return conn
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("no compositor connection accepted before deadline")
	return nil
}

func (c *fakeCompositor) sendTap(t *testing.T, objectID uint32, pressed bool) {
	t.Helper()
	conn := c.connection(t)
	opcode := wlShortcutEventReleased
	if pressed {
		opcode = wlShortcutEventPressed
	}
	// tv_sec_hi, tv_sec_lo, tv_nsec: the real protocol's three-uint32
	// timestamp payload. The client discards it, but sending it keeps
	// this fake's message shape faithful to a real compositor's.
	body := append(wlUint32(0), append(wlUint32(0), wlUint32(0)...)...)
	if err := writeWlMessage(conn, objectID, opcode, body); err != nil {
		t.Fatalf("send shortcut event: %v", err)
	}
}

// TestHyprlandSourceDrivesDetectorDoubleTap wires
// internal/input.NewHyprlandSource straight into a real
// internal/hotkey.Detector and replays two clean physical taps within
// the chord window, asserting the double-tap fires on the second
// release — the acceptance criterion copper-l2z.74 exists to satisfy.
func TestHyprlandSourceDrivesDetectorDoubleTap(t *testing.T) {
	dir := t.TempDir()
	sig := "integration-sig"
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", sig)
	t.Setenv("XDG_RUNTIME_DIR", dir)

	hyprDir := filepath.Join(dir, "hypr", sig)
	if err := os.MkdirAll(hyprDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	newFakeReqServer(t, filepath.Join(hyprDir, ".socket.sock"))

	wlPath := filepath.Join(dir, "wayland-test.sock")
	compositor := newFakeCompositor(t, wlPath)
	t.Setenv("WAYLAND_DISPLAY", wlPath)

	src, err := input.NewHyprlandSource()
	if err != nil {
		t.Fatalf("NewHyprlandSource: %v", err)
	}
	defer func() { _ = src.Close() }()

	shortcutID := compositor.firstShortcutID(t)

	det := hotkey.NewDetector(350 * time.Millisecond)

	feed := func() bool {
		select {
		case ev, ok := <-src.Events():
			if !ok {
				t.Fatalf("Events() channel closed unexpectedly")
			}
			return det.Feed(ev)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for an event")
			return false
		}
	}

	// Tap 1: press, release. Neither edge fires the chord.
	compositor.sendTap(t, shortcutID, true)
	if feed() {
		t.Fatalf("first press fired the chord early")
	}
	compositor.sendTap(t, shortcutID, false)
	if feed() {
		t.Fatalf("first release fired the chord early")
	}

	// Tap 2, well within the 350ms window: press, release. The release
	// must fire. Before copper-l2z.74's fix, the duplicate Pressed event
	// a second shortcut object would have injected here reset Detector's
	// state machine before this point was ever reached.
	compositor.sendTap(t, shortcutID, true)
	if feed() {
		t.Fatalf("second press fired the chord early")
	}
	compositor.sendTap(t, shortcutID, false)
	if !feed() {
		t.Fatalf("second release did not fire the double-tap chord")
	}
}
