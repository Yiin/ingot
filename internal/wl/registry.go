package wl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// fetchGlobals performs the roundtrip: get_registry, then sync as a
// barrier. wl_registry.global events arrive in between; the sync
// callback's done event confirms the compositor has sent every global
// that existed at connect time, so it is safe to stop reading there.
func fetchGlobals(ctx context.Context, conn net.Conn) ([]global, error) {
	deadline := time.Now().Add(defaultProbeTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, err
	}

	if err := writeRequest(conn, displayObjectID, displayOpGetRegistry, registryObjectID); err != nil {
		return nil, fmt.Errorf("wl: get_registry: %w", err)
	}
	if err := writeRequest(conn, displayObjectID, displayOpSync, callbackObjectID); err != nil {
		return nil, fmt.Errorf("wl: sync: %w", err)
	}

	var globals []global
	for {
		h, err := readHeader(conn)
		if err != nil {
			return nil, fmt.Errorf("wl: read header: %w", err)
		}

		payloadLen := int(h.Size) - 8
		if payloadLen < 0 {
			return nil, fmt.Errorf("wl: message size %d smaller than an 8-byte header", h.Size)
		}
		payload := make([]byte, payloadLen)
		if payloadLen > 0 {
			if _, err := io.ReadFull(conn, payload); err != nil {
				return nil, fmt.Errorf("wl: read payload: %w", err)
			}
		}
		dec := &payloadDecoder{buf: payload}

		switch {
		case h.ObjectID == registryObjectID && h.Opcode == registryEventGlobal:
			name, err := dec.uint32()
			if err != nil {
				return nil, fmt.Errorf("wl: decode global.name: %w", err)
			}
			iface, err := dec.string()
			if err != nil {
				return nil, fmt.Errorf("wl: decode global.interface: %w", err)
			}
			version, err := dec.uint32()
			if err != nil {
				return nil, fmt.Errorf("wl: decode global.version: %w", err)
			}
			globals = append(globals, global{Name: name, Interface: iface, Version: version})

		case h.ObjectID == callbackObjectID && h.Opcode == callbackEventDone:
			return globals, nil

		case h.ObjectID == displayObjectID && h.Opcode == displayEventError:
			return nil, errors.New("wl: compositor reported a protocol error")

		default:
			// wl_registry.global_remove, wl_display.delete_id, or anything
			// else this package has no use for. The payload bytes were
			// already consumed above regardless of interpretation, which
			// keeps the stream in sync for the next header.
		}
	}
}
