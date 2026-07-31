package wl

import (
	"bytes"
	"testing"
)

func TestPayloadDecoder_String(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
		want string
	}{
		{"empty", []byte{0, 0, 0, 0}, ""},
		{"one char, no padding needed for header but body pads", []byte{2, 0, 0, 0, 'a', 0, 0, 0}, "a"},
		{"three chars, exactly one word", []byte{4, 0, 0, 0, 'w', 'l', 'r', 0}, "wlr"},
		{"five chars, pads to next word", []byte{6, 0, 0, 0, 'h', 'e', 'l', 'l', 'o', 0, 0, 0}, "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &payloadDecoder{buf: tt.buf}
			got, err := d.string()
			if err != nil {
				t.Fatalf("string(): %v", err)
			}
			if got != tt.want {
				t.Errorf("string() = %q, want %q", got, tt.want)
			}
			if len(d.buf) != 0 {
				t.Errorf("%d bytes left over, want the decoder to consume the whole padded field", len(d.buf))
			}
		})
	}
}

func TestPayloadDecoder_String_Truncated(t *testing.T) {
	// Claims a 100-byte string but the buffer is empty.
	d := &payloadDecoder{buf: []byte{100, 0, 0, 0}}
	if _, err := d.string(); err == nil {
		t.Fatal("string() with an out-of-range length returned nil error")
	}
}

func TestPayloadDecoder_String_HostileLengthDoesNotPanic(t *testing.T) {
	// A length near the uint32 max must not overflow (n+3) into a small,
	// wrongly in-bounds padded value and then slice past the buffer.
	d := &payloadDecoder{buf: []byte{0xfd, 0xff, 0xff, 0xff, 1, 2, 3, 4}}
	if _, err := d.string(); err == nil {
		t.Fatal("string() with a hostile length returned nil error")
	}
}

func TestPayloadDecoder_Uint32_Truncated(t *testing.T) {
	d := &payloadDecoder{buf: []byte{1, 2}}
	if _, err := d.uint32(); err == nil {
		t.Fatal("uint32() with 2 remaining bytes returned nil error")
	}
}

func TestWriteRequest_ReadHeader_Roundtrip(t *testing.T) {
	var buf bytes.Buffer
	if err := writeRequest(&buf, displayObjectID, displayOpGetRegistry, registryObjectID); err != nil {
		t.Fatalf("writeRequest: %v", err)
	}

	h, err := readHeader(&buf)
	if err != nil {
		t.Fatalf("readHeader: %v", err)
	}
	if h.ObjectID != displayObjectID || h.Opcode != displayOpGetRegistry || h.Size != 12 {
		t.Errorf("header = %+v, want {ObjectID:%d Opcode:%d Size:12}", h, displayObjectID, displayOpGetRegistry)
	}

	dec := &payloadDecoder{buf: buf.Bytes()}
	arg, err := dec.uint32()
	if err != nil {
		t.Fatalf("decode arg: %v", err)
	}
	if arg != registryObjectID {
		t.Errorf("arg = %d, want %d", arg, registryObjectID)
	}
}
