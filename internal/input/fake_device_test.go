package input

import (
	"io"
	"os"
	"sync"

	"github.com/holoplot/go-evdev"
)

// fakeDevice is a rawDevice double: it lets tests drive ReadOne with
// scripted events and observe whether Close was called, without needing a
// real device node or root.
type fakeDevice struct {
	codes []evdev.EvCode

	ch        chan *evdev.InputEvent
	closeOnce sync.Once
	closed    chan struct{}

	mu          sync.Mutex
	closeCalled bool
}

func newFakeDevice(codes ...evdev.EvCode) *fakeDevice {
	return &fakeDevice{
		codes:  codes,
		ch:     make(chan *evdev.InputEvent),
		closed: make(chan struct{}),
	}
}

func (f *fakeDevice) CapableEvents(t evdev.EvType) []evdev.EvCode {
	if t != evdev.EV_KEY {
		return nil
	}
	return f.codes
}

func (f *fakeDevice) ReadOne() (*evdev.InputEvent, error) {
	select {
	case ev, ok := <-f.ch:
		if !ok {
			return nil, io.EOF
		}
		return ev, nil
	case <-f.closed:
		return nil, os.ErrClosed
	}
}

func (f *fakeDevice) NonBlock() error {
	return nil
}

func (f *fakeDevice) Close() error {
	f.mu.Lock()
	f.closeCalled = true
	f.mu.Unlock()
	f.closeOnce.Do(func() { close(f.closed) })
	return nil
}

func (f *fakeDevice) wasClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closeCalled
}

// push delivers ev to a pending ReadOne, or drops it silently if the
// device has been closed in the meantime.
func (f *fakeDevice) push(ev evdev.InputEvent) {
	select {
	case f.ch <- &ev:
	case <-f.closed:
	}
}

// fakeOpener implements openFunc backed by a path -> fakeDevice registry,
// with optional per-path permission-denial for EACCES tests.
type fakeOpener struct {
	mu         sync.Mutex
	devices    map[string]*fakeDevice
	permDenied map[string]bool
}

func newFakeOpener() *fakeOpener {
	return &fakeOpener{
		devices:    make(map[string]*fakeDevice),
		permDenied: make(map[string]bool),
	}
}

func (o *fakeOpener) register(path string, d *fakeDevice) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.devices[path] = d
}

func (o *fakeOpener) denyPermission(path string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.permDenied[path] = true
}

func (o *fakeOpener) open(path string) (rawDevice, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.permDenied[path] {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrPermission}
	}
	d, ok := o.devices[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return d, nil
}
