package wl

import (
	"encoding/binary"
	"errors"
	"io"
)

// Object IDs this package allocates for the short-lived roundtrip in
// fetchGlobals. wl_display is always object 1 by protocol definition; the
// other two are new_ids we hand out ourselves, so any fixed values that
// don't collide with 1 are fine.
const (
	displayObjectID  uint32 = 1
	registryObjectID uint32 = 2
	callbackObjectID uint32 = 3
)

// Opcodes for the handful of requests and events this package speaks.
const (
	displayOpSync        uint16 = 0
	displayOpGetRegistry uint16 = 1

	displayEventError    uint16 = 0
	displayEventDeleteID uint16 = 1

	registryEventGlobal       uint16 = 0
	registryEventGlobalRemove uint16 = 1

	callbackEventDone uint16 = 0
)

// global is one entry read from a wl_registry.global event: a name the
// compositor advertises, its interface, and the version it supports.
type global struct {
	Name      uint32
	Interface string
	Version   uint32
}

// header is the 8-byte prefix on every Wayland wire message: which object
// sent it, its opcode, and the total message size including this header.
type header struct {
	ObjectID uint32
	Opcode   uint16
	Size     uint16
}

// readHeader reads one message header. The wire protocol has no
// framing beyond this: a short read here means the connection is gone.
func readHeader(r io.Reader) (header, error) {
	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return header{}, err
	}
	return header{
		ObjectID: binary.LittleEndian.Uint32(buf[0:4]),
		Opcode:   binary.LittleEndian.Uint16(buf[4:6]),
		Size:     binary.LittleEndian.Uint16(buf[6:8]),
	}, nil
}

// writeRequest writes one Wayland wire request whose only argument is a
// single uint32 (an object or new_id) — the shape of both get_registry and
// sync, the only two requests this package ever sends.
func writeRequest(w io.Writer, objectID uint32, opcode uint16, arg uint32) error {
	const size = 8 + 4
	var buf [size]byte
	binary.LittleEndian.PutUint32(buf[0:4], objectID)
	binary.LittleEndian.PutUint16(buf[4:6], opcode)
	binary.LittleEndian.PutUint16(buf[6:8], size)
	binary.LittleEndian.PutUint32(buf[8:12], arg)
	_, err := w.Write(buf[:])
	return err
}

// payloadDecoder reads fixed-width and string arguments out of one
// message's payload bytes, in the order the protocol defines them for that
// message. It never reads past what it was given: a message this package
// doesn't care about is simply not decoded, only skipped by its caller.
type payloadDecoder struct {
	buf []byte
}

var errTruncated = errors.New("wl: truncated message argument")

func (d *payloadDecoder) uint32() (uint32, error) {
	if len(d.buf) < 4 {
		return 0, errTruncated
	}
	v := binary.LittleEndian.Uint32(d.buf[:4])
	d.buf = d.buf[4:]
	return v, nil
}

// string decodes a Wayland wire string: a uint32 length that includes the
// trailing NUL, the bytes themselves, then zero padding out to a 4-byte
// boundary.
func (d *payloadDecoder) string() (string, error) {
	n, err := d.uint32()
	if err != nil {
		return "", err
	}
	// Bound n against the remaining buffer before it's used to compute
	// padded and slice below: a corrupt or hostile n near uint32's max
	// would otherwise overflow (n+3) and wrap to a small, wrongly
	// "in bounds" padded value. Compare as uint64 rather than int: on a
	// 32-bit platform int(n) itself can go negative for n above
	// 1<<31, which would pass an int comparison it should fail.
	if uint64(n) > uint64(len(d.buf)) {
		return "", errTruncated
	}
	padded := int((n + 3) &^ 3)
	if padded > len(d.buf) {
		return "", errTruncated
	}
	s := ""
	if n > 0 {
		s = string(d.buf[:n-1]) // drop the trailing NUL
	}
	d.buf = d.buf[padded:]
	return s, nil
}
