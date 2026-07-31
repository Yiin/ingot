package input

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrGlobalShortcutsUnsupported is returned by dialGlobalShortcuts when
// the compositor's registry never advertises
// hyprland_global_shortcuts_manager_v1 — a non-Hyprland compositor, or a
// Hyprland build old enough to lack the protocol.
var ErrGlobalShortcutsUnsupported = errors.New("input: hyprland: compositor does not advertise hyprland_global_shortcuts_manager_v1")

// hyprlandGlobalShortcutsInterface is the Wayland global this client
// binds directly, with no XDG portal and no app-id gate — the actual
// activation-delivery mechanism behind Hyprland's "global" bind
// dispatcher. Live verification (copper-l2z.50) proved the alternative,
// watching .socket2.sock for a "globalshortcut>>" line, never fires:
// that dispatcher only reaches a client registered through
// org.freedesktop.portal.GlobalShortcuts, which rejects a plain
// non-sandboxed Go binary. This protocol has neither restriction.
const hyprlandGlobalShortcutsInterface = "hyprland_global_shortcuts_manager_v1"

// gsManagerSupportedVersion is the highest version of the manager
// interface this client understands. It never binds higher than this
// even if the compositor advertises more, since binding at a version
// implies understanding every event that version can send.
const gsManagerSupportedVersion uint32 = 1

// gsConnectTimeout bounds the registry roundtrip and the setup requests
// (bind, register_shortcut) against a compositor that accepts the
// connection but never replies. It is not applied once setup finishes:
// the event stream that follows has no fixed cadence.
const gsConnectTimeout = 2 * time.Second

// Object ids this client allocates. wl_display is always 1 by protocol
// definition; the rest are new_ids handed out during the fixed setup
// sequence, so fixed values suffice — this client never creates or
// destroys objects beyond this initial set.
const (
	gsDisplayID  uint32 = 1
	gsRegistryID uint32 = 2
	gsCallbackID uint32 = 3
	gsManagerID  uint32 = 4
)

// Opcodes for every request and event this client speaks: core
// wl_display/wl_registry, plus hyprland_global_shortcuts_manager_v1 and
// hyprland_global_shortcut_v1 per
// /usr/include/hyprland/protocols/hyprland-global-shortcuts-v1.hpp
// (hyprland 0.56.0 on the machine this was verified against). Request
// and event opcodes are assigned by declaration order within each
// interface, per the Wayland wire protocol convention.
const (
	gsDisplayOpSync        uint16 = 0
	gsDisplayOpGetRegistry uint16 = 1
	gsDisplayEventError    uint16 = 0

	gsRegistryOpBind      uint16 = 0
	gsRegistryEventGlobal uint16 = 0

	gsCallbackEventDone uint16 = 0

	gsManagerOpRegisterShortcut uint16 = 0

	// hyprland_global_shortcut_v1 has exactly two wire events: pressed
	// and released. The header's sendPressedRaw/sendReleasedRaw methods
	// are not a third and fourth opcode — hyprwayland-scanner generates
	// an unchecked-argument "Raw" C++ overload for every event on every
	// interface it scans (confirmed against wl_registry in the same
	// header set: wl_registry.global is one wire event, but the
	// generated class exposes both sendGlobal and sendGlobalRaw). Both
	// names send the same two wire opcodes below.
	gsShortcutEventPressed  uint16 = 0
	gsShortcutEventReleased uint16 = 1
)

// gsShortcutSlot identifies one of the four chord edges this client
// registers, so an incoming event can be mapped back to (side, edge)
// by object id alone, with no string parsing on the hot path.
type gsShortcutSlot int

const (
	gsSlotLDown gsShortcutSlot = iota
	gsSlotLUp
	gsSlotRDown
	gsSlotRUp
)

// gsShortcutDef pairs one slot with the object id this client assigns
// it and the id/description strings sent to register_shortcut. The id
// strings must match hyprlandNames exactly: they are also the names
// bound to physical keys over .socket.sock, and Hyprland correlates the
// two registrations by that string.
type gsShortcutDef struct {
	objectID    uint32
	id          string
	description string
	slot        gsShortcutSlot
}

func gsShortcutDefs() []gsShortcutDef {
	return []gsShortcutDef{
		{objectID: 5, id: hyprlandNames.lDown, description: "Ingot: left Shift pressed", slot: gsSlotLDown},
		{objectID: 6, id: hyprlandNames.lUp, description: "Ingot: left Shift released", slot: gsSlotLUp},
		{objectID: 7, id: hyprlandNames.rDown, description: "Ingot: right Shift pressed", slot: gsSlotRDown},
		{objectID: 8, id: hyprlandNames.rUp, description: "Ingot: right Shift released", slot: gsSlotRUp},
	}
}

// gsAppID is the app_id argument register_shortcut requires. It is
// otherwise unused: Ingot has no XDG desktop-entry gate to satisfy here,
// unlike the portal path this protocol replaces.
const gsAppID = "lt.yiin.ingot"

// wlDialFunc opens the Wayland client connection this package binds the
// global-shortcuts protocol over. Tests substitute one that dials a fake
// compositor listener instead of the real compositor socket.
type wlDialFunc func(ctx context.Context) (net.Conn, error)

// dialWaylandSocket connects to the compositor named by WAYLAND_DISPLAY,
// resolved against XDG_RUNTIME_DIR exactly as internal/wl.dial does.
// internal/wl does not export that helper — it is scoped to a single
// probe roundtrip — so this is a deliberate, small duplication rather
// than a shared dependency between two otherwise-independent packages.
func dialWaylandSocket(ctx context.Context) (net.Conn, error) {
	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		display = "wayland-0"
	}

	path := display
	if !filepath.IsAbs(path) {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			return nil, errors.New("input: hyprland: XDG_RUNTIME_DIR not set")
		}
		path = filepath.Join(runtimeDir, display)
	}

	d := net.Dialer{Timeout: gsConnectTimeout}
	return d.DialContext(ctx, "unix", path)
}

// globalShortcutsClient owns one persistent connection to the Wayland
// compositor: it binds hyprland_global_shortcuts_manager_v1, registers
// the four chord shortcuts, and delivers their pressed/released
// events to onEvent from a dedicated read goroutine until Close.
type globalShortcutsClient struct {
	conn net.Conn

	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
}

// dialGlobalShortcuts performs the full setup roundtrip — connect,
// discover the manager global, bind it, register every shortcut in
// defs — and starts the event read loop. onEvent is invoked from that
// goroutine for every pressed/released event and must not
// block. logger receives one Warn if the connection drops on its own
// (compositor restart, protocol error) rather than via Close — nil
// disables that log.
func dialGlobalShortcuts(ctx context.Context, dial wlDialFunc, defs []gsShortcutDef, onEvent func(gsShortcutSlot, bool), logger *slog.Logger) (*globalShortcutsClient, error) {
	conn, err := dial(ctx)
	if err != nil {
		return nil, fmt.Errorf("input: hyprland: dial wayland socket: %w", err)
	}

	deadline := time.Now().Add(gsConnectTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return nil, err
	}

	if err := setupGlobalShortcuts(conn, defs); err != nil {
		conn.Close()
		return nil, err
	}

	// The setup roundtrip is over; everything from here on is an
	// unsolicited event with no fixed cadence, so the deadline that
	// bounded setup must not keep bounding the read loop.
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, err
	}

	c := &globalShortcutsClient{conn: conn}
	c.wg.Add(1)
	go c.readEvents(defs, onEvent, logger)
	return c, nil
}

func setupGlobalShortcuts(conn net.Conn, defs []gsShortcutDef) error {
	if err := writeGsUint32Request(conn, gsDisplayID, gsDisplayOpGetRegistry, gsRegistryID); err != nil {
		return fmt.Errorf("input: hyprland: get_registry: %w", err)
	}
	if err := writeGsUint32Request(conn, gsDisplayID, gsDisplayOpSync, gsCallbackID); err != nil {
		return fmt.Errorf("input: hyprland: sync: %w", err)
	}

	var managerName, managerVersion uint32
	found := false

registryLoop:
	for {
		h, payload, err := readGsMessage(conn)
		if err != nil {
			return fmt.Errorf("input: hyprland: read registry: %w", err)
		}
		dec := &gsDecoder{buf: payload}

		switch {
		case h.objectID == gsRegistryID && h.opcode == gsRegistryEventGlobal:
			name, err := dec.uint32()
			if err != nil {
				return fmt.Errorf("input: hyprland: decode global.name: %w", err)
			}
			iface, err := dec.string()
			if err != nil {
				return fmt.Errorf("input: hyprland: decode global.interface: %w", err)
			}
			version, err := dec.uint32()
			if err != nil {
				return fmt.Errorf("input: hyprland: decode global.version: %w", err)
			}
			if iface == hyprlandGlobalShortcutsInterface {
				managerName, managerVersion, found = name, version, true
			}

		case h.objectID == gsCallbackID && h.opcode == gsCallbackEventDone:
			break registryLoop

		case h.objectID == gsDisplayID && h.opcode == gsDisplayEventError:
			return errors.New("input: hyprland: compositor reported a protocol error during registry roundtrip")
		}
	}

	if !found {
		return ErrGlobalShortcutsUnsupported
	}

	bindVersion := managerVersion
	if bindVersion > gsManagerSupportedVersion {
		bindVersion = gsManagerSupportedVersion
	}

	bind := &gsEncoder{}
	bind.uint32(managerName)
	bind.string(hyprlandGlobalShortcutsInterface)
	bind.uint32(bindVersion)
	bind.uint32(gsManagerID)
	if err := writeGsMessage(conn, gsRegistryID, gsRegistryOpBind, bind.buf); err != nil {
		return fmt.Errorf("input: hyprland: bind: %w", err)
	}

	for _, def := range defs {
		reg := &gsEncoder{}
		reg.uint32(def.objectID)
		reg.string(def.id)
		reg.string(gsAppID)
		reg.string(def.description)
		reg.string("") // trigger_description: the key combo is described to the user elsewhere; Hyprland only surfaces this in its own shortcut-list UI.
		if err := writeGsMessage(conn, gsManagerID, gsManagerOpRegisterShortcut, reg.buf); err != nil {
			return fmt.Errorf("input: hyprland: register_shortcut %s: %w", def.id, err)
		}
	}

	return nil
}

// readEvents drains the connection until it errs, including the
// deliberate close from Close — which is what unblocks the pending
// read. Any other exit (compositor restart, a posted protocol error
// such as already_taken) is logged: the caller would otherwise see this
// Source as healthy while it silently stopped delivering activations.
func (c *globalShortcutsClient) readEvents(defs []gsShortcutDef, onEvent func(gsShortcutSlot, bool), logger *slog.Logger) {
	defer c.wg.Done()

	slots := make(map[uint32]gsShortcutSlot, len(defs))
	for _, def := range defs {
		slots[def.objectID] = def.slot
	}

	for {
		h, _, err := readGsMessage(c.conn)
		if err != nil {
			c.mu.Lock()
			deliberate := c.closed
			c.mu.Unlock()
			if !deliberate && logger != nil {
				logger.Warn("input: hyprland: global-shortcuts connection dropped", "err", err)
			}
			return
		}

		slot, ok := slots[h.objectID]
		if !ok {
			continue
		}
		switch h.opcode {
		case gsShortcutEventPressed:
			onEvent(slot, true)
		case gsShortcutEventReleased:
			onEvent(slot, false)
		}
	}
}

// Close tears down the connection, which the compositor treats as
// destroying every object bound over it — no explicit destroy request
// is needed. It waits for the read goroutine to observe the close
// before returning, matching hyprlandSource's own Close/Pause
// discipline.
func (c *globalShortcutsClient) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	err := c.conn.Close()
	c.wg.Wait()
	return err
}

// gsHeader is the 8-byte prefix on every Wayland wire message.
type gsHeader struct {
	objectID uint32
	opcode   uint16
	size     uint16
}

func readGsMessage(r io.Reader) (gsHeader, []byte, error) {
	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return gsHeader{}, nil, err
	}
	h := gsHeader{
		objectID: binary.LittleEndian.Uint32(buf[0:4]),
		opcode:   binary.LittleEndian.Uint16(buf[4:6]),
		size:     binary.LittleEndian.Uint16(buf[6:8]),
	}

	payloadLen := int(h.size) - 8
	if payloadLen < 0 {
		return gsHeader{}, nil, fmt.Errorf("input: hyprland: message size %d smaller than an 8-byte header", h.size)
	}
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return gsHeader{}, nil, err
		}
	}
	return h, payload, nil
}

// writeGsUint32Request writes a request whose only argument is a single
// uint32 — the shape of both get_registry and sync.
func writeGsUint32Request(w io.Writer, objectID uint32, opcode uint16, arg uint32) error {
	e := &gsEncoder{}
	e.uint32(arg)
	return writeGsMessage(w, objectID, opcode, e.buf)
}

func writeGsMessage(w io.Writer, objectID uint32, opcode uint16, body []byte) error {
	size := 8 + len(body)
	if size > 0xFFFF {
		return fmt.Errorf("input: hyprland: message too large (%d bytes)", size)
	}
	buf := make([]byte, 8, size)
	binary.LittleEndian.PutUint32(buf[0:4], objectID)
	binary.LittleEndian.PutUint16(buf[4:6], opcode)
	binary.LittleEndian.PutUint16(buf[6:8], uint16(size))
	buf = append(buf, body...)
	_, err := w.Write(buf)
	return err
}

// gsEncoder appends wire-format request arguments in the order the
// protocol defines them for one message.
type gsEncoder struct {
	buf []byte
}

func (e *gsEncoder) uint32(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	e.buf = append(e.buf, b[:]...)
}

// string encodes a Wayland wire string: a uint32 length that includes
// the trailing NUL, the bytes themselves, then zero padding out to a
// 4-byte boundary.
func (e *gsEncoder) string(s string) {
	e.uint32(uint32(len(s) + 1))
	e.buf = append(e.buf, s...)
	e.buf = append(e.buf, 0)
	for len(e.buf)%4 != 0 {
		e.buf = append(e.buf, 0)
	}
}

// gsDecoder reads fixed-width and string arguments out of one message's
// payload bytes, in the order the protocol defines them.
type gsDecoder struct {
	buf []byte
}

var errGsTruncated = errors.New("input: hyprland: truncated message argument")

func (d *gsDecoder) uint32() (uint32, error) {
	if len(d.buf) < 4 {
		return 0, errGsTruncated
	}
	v := binary.LittleEndian.Uint32(d.buf[:4])
	d.buf = d.buf[4:]
	return v, nil
}

func (d *gsDecoder) string() (string, error) {
	n, err := d.uint32()
	if err != nil {
		return "", err
	}
	// Bound n against the remaining buffer before it is used to compute
	// padded below: a corrupt or hostile n near uint32's max would
	// otherwise overflow (n+3) and wrap to a small, wrongly "in bounds"
	// padded value.
	if uint64(n) > uint64(len(d.buf)) {
		return "", errGsTruncated
	}
	padded := int((n + 3) &^ 3)
	if padded > len(d.buf) {
		return "", errGsTruncated
	}
	s := ""
	if n > 0 {
		s = string(d.buf[:n-1]) // drop the trailing NUL
	}
	d.buf = d.buf[padded:]
	return s, nil
}
