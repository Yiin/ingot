package input

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/holoplot/go-evdev"
	"golang.org/x/sys/unix"
)

// defaultDir is where the kernel creates evdev device nodes.
const defaultDir = "/dev/input"

// debounceDelay is how long the watcher waits after the last inotify event
// before rescanning. udev sets device permissions via a chmod (IN_ATTRIB)
// that lands shortly after IN_CREATE; rescanning too eagerly opens a
// device it cannot yet read.
const debounceDelay = 150 * time.Millisecond

// deviceHandle pairs an open device with the path it was opened from, so a
// stale entry can be identified and dropped from the device map.
type deviceHandle struct {
	path string
	dev  rawDevice
}

// evdevSource is the real, evdev-backed Source.
type evdevSource struct {
	dir    string
	open   openFunc
	logger *slog.Logger

	events chan Event
	stopCh chan struct{}
	wg     sync.WaitGroup

	inotifyFile *os.File

	mu      sync.Mutex
	devices map[string]*deviceHandle

	closeOnce sync.Once
}

// NewSource opens every /dev/input/event* device whose EV_KEY capabilities
// include a Shift key or the left mouse button, and watches for devices
// added or removed afterward. It never fails outright because one device
// could not be opened (e.g. a permission-denied device node); it only
// fails if it cannot enumerate or watch the directory at all.
func NewSource() (Source, error) {
	return newSource(defaultDir, openReal, slog.Default())
}

func newSource(dir string, open openFunc, logger *slog.Logger) (*evdevSource, error) {
	// IN_NONBLOCK is what lets Close unblock the pending inotify read by
	// closing s.inotifyFile: a blocking-mode fd would not respond to that.
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("input: inotify_init1: %w", err)
	}
	// IN_ATTRIB matters as much as IN_CREATE: udev creates the node first
	// and chmods it into readability afterward, so a create-only watch
	// races the permission grant. IN_DELETE surfaces unplug even though a
	// read on the still-open fd also reports removal via ENODEV.
	if _, err := unix.InotifyAddWatch(fd, dir, unix.IN_CREATE|unix.IN_ATTRIB|unix.IN_DELETE); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("input: watch %s: %w", dir, err)
	}

	if logger == nil {
		logger = slog.Default()
	}

	s := &evdevSource{
		dir:         dir,
		open:        open,
		logger:      logger,
		events:      make(chan Event, 64),
		stopCh:      make(chan struct{}),
		inotifyFile: os.NewFile(uintptr(fd), "inotify:"+dir),
		devices:     make(map[string]*deviceHandle),
	}

	s.rescan()

	raw := make(chan struct{})
	s.wg.Add(2)
	go s.readInotify(raw)
	go s.watchLoop(raw)

	return s, nil
}

func (s *evdevSource) Events() <-chan Event {
	return s.events
}

func (s *evdevSource) Close() error {
	s.closeOnce.Do(func() {
		close(s.stopCh)
		_ = s.inotifyFile.Close()
		s.wg.Wait()
		close(s.events)
	})
	return nil
}

// rescan lists the watched directory, opens and filters newly appeared
// devices, and drops tracked devices whose node disappeared.
func (s *evdevSource) rescan() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		s.logger.Warn("input: read device dir", "dir", s.dir, "err", err)
		return
	}

	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "event") {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		seen[path] = true

		s.mu.Lock()
		_, tracked := s.devices[path]
		s.mu.Unlock()
		if tracked {
			continue
		}

		dev, err := s.open(path)
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				s.logger.Warn("input: permission denied opening device, will retry", "path", path, "err", err)
			} else {
				s.logger.Debug("input: could not open device", "path", path, "err", err)
			}
			continue
		}

		if !hasChordCapability(dev.CapableEvents(evdev.EV_KEY)) {
			_ = dev.Close()
			continue
		}

		// Non-blocking mode is what lets Close unblock a pending ReadOne
		// by closing the device: a plain blocking read on a device that
		// never sends another event would otherwise never return.
		if err := dev.NonBlock(); err != nil {
			s.logger.Warn("input: could not set device non-blocking", "path", path, "err", err)
		}

		h := &deviceHandle{path: path, dev: dev}
		s.mu.Lock()
		s.devices[path] = h
		s.mu.Unlock()

		s.wg.Add(1)
		go s.readDevice(h)
	}

	s.mu.Lock()
	for path, h := range s.devices {
		if !seen[path] {
			_ = h.dev.Close()
			delete(s.devices, path)
		}
	}
	s.mu.Unlock()
}

// readDevice pumps reduced events from a single device until it errors —
// including ENODEV when the device is unplugged — or Close stops the
// source. A per-device failure only removes that device; it never brings
// down the rest of the source.
func (s *evdevSource) readDevice(h *deviceHandle) {
	defer s.wg.Done()
	for {
		ev, err := h.dev.ReadOne()
		if err != nil {
			s.mu.Lock()
			if s.devices[h.path] == h {
				delete(s.devices, h.path)
			}
			s.mu.Unlock()
			return
		}

		out, ok := reduce(ev)
		if !ok {
			continue
		}

		select {
		case s.events <- out:
		case <-s.stopCh:
			return
		}
	}
}

// readInotify blocks on the inotify fd and emits a signal for every batch
// of events read. It carries no event details: any signal is enough to
// trigger a debounced rescan. Closing s.inotifyFile from Close unblocks
// the pending read.
func (s *evdevSource) readInotify(out chan<- struct{}) {
	defer s.wg.Done()
	defer close(out)

	buf := make([]byte, 4096)
	for {
		n, err := s.inotifyFile.Read(buf)
		if err != nil {
			return
		}
		if n == 0 {
			continue
		}
		select {
		case out <- struct{}{}:
		case <-s.stopCh:
			return
		}
	}
}

// watchLoop debounces inotify signals and triggers rescans, then closes
// every tracked device once told to stop so their read goroutines unblock.
func (s *evdevSource) watchLoop(raw <-chan struct{}) {
	defer s.wg.Done()

	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	for {
		select {
		case <-s.stopCh:
			s.closeAllDevices()
			return

		case _, ok := <-raw:
			if !ok {
				s.closeAllDevices()
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(debounceDelay)

		case <-timer.C:
			select {
			case <-s.stopCh:
				s.closeAllDevices()
				return
			default:
			}
			s.rescan()
		}
	}
}

func (s *evdevSource) closeAllDevices() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for path, h := range s.devices {
		_ = h.dev.Close()
		delete(s.devices, path)
	}
}
