package wl

import (
	"context"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

// serveFixture runs a fake compositor over conn: it discards the two
// incoming requests (get_registry, sync — 12 bytes each) and then writes
// the fixture bytes as its reply, exactly mirroring what a real
// compositor's wire traffic looks like from fetchGlobals' point of view.
func serveFixture(t *testing.T, conn net.Conn, fixture []byte) {
	t.Helper()
	go func() {
		defer func() { _ = conn.Close() }()
		var discard [24]byte
		if _, err := io.ReadFull(conn, discard[:]); err != nil {
			return
		}
		_, _ = conn.Write(fixture)
	}()
}

// TestFetchGlobals_RecordedRegistryDump replays a wl_registry roundtrip
// captured live from Hyprland 0.56 (internal/wl/testdata/registry_dump.bin)
// and checks that the interfaces the acceptance criterion names come out
// at the versions Hyprland actually advertised when the fixture was
// recorded.
func TestFetchGlobals_RecordedRegistryDump(t *testing.T) {
	fixture, err := os.ReadFile("testdata/registry_dump.bin")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	serveFixture(t, server, fixture)

	globals, err := fetchGlobals(context.Background(), client)
	if err != nil {
		t.Fatalf("fetchGlobals: %v", err)
	}

	versions := make(map[string]uint32, len(globals))
	for _, g := range globals {
		versions[g.Interface] = g.Version
	}

	want := map[string]uint32{
		extDataControlManagerInterface:  1,
		wlrDataControlManagerInterface:  2,
		wlrLayerShellInterface:          5,
		virtualKeyboardManagerInterface: 1,
	}
	for iface, wantVersion := range want {
		got, ok := versions[iface]
		if !ok {
			t.Errorf("fixture missing global %q, have %d globals total", iface, len(globals))
			continue
		}
		if got != wantVersion {
			t.Errorf("%s version = %d, want %d", iface, got, wantVersion)
		}
	}

	if len(globals) < 10 {
		t.Errorf("only %d globals parsed from a real registry dump, expected dozens", len(globals))
	}
}

// TestFetchGlobals_ProtocolError makes sure a wl_display.error event is
// treated as a hard failure of the roundtrip rather than silently
// ignored, since Probe relies on fetchGlobals returning an error to know
// to degrade.
func TestFetchGlobals_ProtocolError(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()

	go func() {
		defer func() { _ = server.Close() }()
		var discard [24]byte
		if _, err := io.ReadFull(server, discard[:]); err != nil {
			return
		}
		// wl_display(1).error(object_id=1, code=0, message="boom")
		msg := []byte{
			1, 0, 0, 0, // object_id
			0, 0, // opcode 0 (error)
			24, 0, // size
			1, 0, 0, 0, // error's object_id argument
			0, 0, 0, 0, // code
			5, 0, 0, 0, // message length incl NUL
			'b', 'o', 'o', 'm', 0, 0, 0, 0, // "boom\0" padded to 8
		}
		_, _ = server.Write(msg)
	}()

	if _, err := fetchGlobals(context.Background(), client); err == nil {
		t.Fatal("fetchGlobals returned nil error for a wl_display.error event")
	}
}

// TestFetchGlobals_ContextDeadline proves a compositor that accepts the
// connection but never replies doesn't hang fetchGlobals forever.
func TestFetchGlobals_ContextDeadline(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := fetchGlobals(ctx, client)
	if err == nil {
		t.Fatal("fetchGlobals returned nil error against a silent peer")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("fetchGlobals took %v, want it bounded by the context deadline", elapsed)
	}
}
