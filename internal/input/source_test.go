package input

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/holoplot/go-evdev"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitForEvent(t *testing.T, ch <-chan Event, timeout time.Duration) Event {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("events channel closed while waiting for an event")
		}
		return ev
	case <-time.After(timeout):
		t.Fatal("timed out waiting for event")
		return Event{}
	}
}

func requireNoEvent(t *testing.T, ch <-chan Event, within time.Duration) {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if ok {
			t.Fatalf("unexpected event delivered: %+v", ev)
		}
	case <-time.After(within):
	}
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestSource_InitialScan_MatchesAndFilters(t *testing.T) {
	dir := t.TempDir()
	shiftPath := filepath.Join(dir, "event0")
	plainPath := filepath.Join(dir, "event1")
	comboPath := filepath.Join(dir, "event2")

	for _, p := range []string{shiftPath, plainPath, comboPath} {
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatalf("seed device node %s: %v", p, err)
		}
	}

	opener := newFakeOpener()
	shiftDev := newFakeDevice(evdev.KEY_LEFTSHIFT)
	plainDev := newFakeDevice(evdev.KEY_A, evdev.KEY_ENTER)
	comboDev := newFakeDevice(evdev.KEY_LEFTSHIFT, evdev.EvCode(evdev.BTN_LEFT))
	opener.register(shiftPath, shiftDev)
	opener.register(plainPath, plainDev)
	opener.register(comboPath, comboDev)

	src, err := newSource(dir, opener.open, discardLogger())
	if err != nil {
		t.Fatalf("newSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	// The non-matching device must be closed and never tracked.
	waitUntil(t, time.Second, plainDev.wasClosed)

	// Both matching devices, including the shift+BTN_LEFT combo, must be
	// tracked and forward events.
	shiftDev.push(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTSHIFT, Value: 1})
	ev := waitForEvent(t, src.Events(), time.Second)
	if ev.Kind != KindShift || !ev.Pressed {
		t.Errorf("shift device event = %+v, want pressed KindShift", ev)
	}

	comboDev.push(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.EvCode(evdev.BTN_LEFT), Value: 1})
	ev = waitForEvent(t, src.Events(), time.Second)
	if ev.Kind != KindPointerButton || !ev.Pressed {
		t.Errorf("combo device event = %+v, want pressed KindPointerButton", ev)
	}
}

func TestSource_EACCES_DoesNotFailSource(t *testing.T) {
	dir := t.TempDir()
	deniedPath := filepath.Join(dir, "event0")
	okPath := filepath.Join(dir, "event1")

	for _, p := range []string{deniedPath, okPath} {
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatalf("seed device node %s: %v", p, err)
		}
	}

	opener := newFakeOpener()
	opener.denyPermission(deniedPath)
	okDev := newFakeDevice(evdev.KEY_RIGHTSHIFT)
	opener.register(okPath, okDev)

	src, err := newSource(dir, opener.open, discardLogger())
	if err != nil {
		t.Fatalf("newSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	okDev.push(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_RIGHTSHIFT, Value: 1})
	ev := waitForEvent(t, src.Events(), time.Second)
	if ev.Kind != KindShift {
		t.Errorf("event = %+v, want KindShift from the readable device", ev)
	}
}

func TestSource_EventReduction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "event0")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("seed device node: %v", err)
	}

	opener := newFakeOpener()
	dev := newFakeDevice(evdev.KEY_LEFTSHIFT, evdev.KEY_A)
	opener.register(path, dev)

	src, err := newSource(dir, opener.open, discardLogger())
	if err != nil {
		t.Fatalf("newSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	// EV_SYN must be dropped entirely.
	dev.push(evdev.InputEvent{Type: evdev.EV_SYN, Code: 0, Value: 0})
	// Autorepeat (Value 2) must be dropped.
	dev.push(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_A, Value: 2})
	// An ordinary key must surface as KindOther with its scancode zeroed.
	dev.push(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_A, Value: 1})

	ev := waitForEvent(t, src.Events(), time.Second)
	if ev.Kind != KindOther || ev.Code != 0 || !ev.Pressed {
		t.Errorf("ordinary key event = %+v, want {KindOther, Code:0, Pressed:true}", ev)
	}

	dev.push(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTSHIFT, Value: 0})
	ev = waitForEvent(t, src.Events(), time.Second)
	if ev.Kind != KindShift || ev.Code != uint16(evdev.KEY_LEFTSHIFT) || ev.Pressed {
		t.Errorf("shift release event = %+v, want {KindShift, Code:%d, Pressed:false}", ev, evdev.KEY_LEFTSHIFT)
	}
}

func TestSource_Hotplug_Add(t *testing.T) {
	dir := t.TempDir()

	opener := newFakeOpener()
	src, err := newSource(dir, opener.open, discardLogger())
	if err != nil {
		t.Fatalf("newSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	path := filepath.Join(dir, "event0")
	dev := newFakeDevice(evdev.KEY_LEFTSHIFT)
	opener.register(path, dev)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("create device node: %v", err)
	}

	// Debounce is ~150ms; allow generous headroom for the rescan.
	waitUntil(t, 2*time.Second, func() bool {
		dev.push(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTSHIFT, Value: 1})
		select {
		case ev := <-src.Events():
			return ev.Kind == KindShift
		case <-time.After(50 * time.Millisecond):
			return false
		}
	})
}

func TestSource_Hotplug_Remove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "event0")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("seed device node: %v", err)
	}

	opener := newFakeOpener()
	dev := newFakeDevice(evdev.KEY_LEFTSHIFT)
	opener.register(path, dev)

	src, err := newSource(dir, opener.open, discardLogger())
	if err != nil {
		t.Fatalf("newSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	waitUntil(t, time.Second, func() bool {
		src.mu.Lock()
		_, tracked := src.devices[path]
		src.mu.Unlock()
		return tracked
	})

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove device node: %v", err)
	}

	waitUntil(t, 2*time.Second, dev.wasClosed)

	src.mu.Lock()
	_, stillTracked := src.devices[path]
	src.mu.Unlock()
	if stillTracked {
		t.Error("device still tracked after its node was removed")
	}
}

func TestSource_Close(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "event0")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("seed device node: %v", err)
	}

	opener := newFakeOpener()
	dev := newFakeDevice(evdev.KEY_LEFTSHIFT)
	opener.register(path, dev)

	src, err := newSource(dir, opener.open, discardLogger())
	if err != nil {
		t.Fatalf("newSource: %v", err)
	}

	waitUntil(t, time.Second, func() bool {
		src.mu.Lock()
		_, tracked := src.devices[path]
		src.mu.Unlock()
		return tracked
	})

	if err := src.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Close must be idempotent.
	if err := src.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	if !dev.wasClosed() {
		t.Error("underlying device was not closed")
	}

	select {
	case _, ok := <-src.Events():
		if ok {
			t.Error("events channel yielded a value after Close")
		}
	case <-time.After(time.Second):
		t.Fatal("events channel was not closed")
	}
}
