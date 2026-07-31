package fsstore

import (
	"errors"
	"io/fs"
	"time"

	"github.com/Yiin/ingot/internal/store"
	"github.com/fsnotify/fsnotify"
)

// watchDebounce coalesces a burst of filesystem events into one reload
// attempt per settled path — editors write in bursts, and some truncate
// before writing, each step its own event.
const watchDebounce = 200 * time.Millisecond

// Watcher is the seam over a filesystem-change notifier for
// Paths.Projects — real fsnotify in production, an injectable fake in
// tests, per the same seam convention as fsx.FS. Events delivers the
// absolute path of every directory entry a Create, Write, Rename, or
// Remove touched; the Store itself Stats each settled path to decide
// what actually happened, so Events need carry nothing beyond the path.
// Close stops delivery and releases any OS resources.
type Watcher interface {
	Events() <-chan string
	Close() error
}

// newFsnotifyWatcher is Options.NewWatcher's default: a Watcher backed
// by a real fsnotify.Watcher on dir.
func newFsnotifyWatcher(dir string) (Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fw.Add(dir); err != nil {
		_ = fw.Close()
		return nil, err
	}
	w := &fsnotifyWatcher{fw: fw, events: make(chan string, 64)}
	go w.pump()
	return w, nil
}

type fsnotifyWatcher struct {
	fw     *fsnotify.Watcher
	events chan string
}

// watchOps is the subset of fsnotify's operations invariant 14 names:
// Create, Write, Rename, Remove.
const watchOps = fsnotify.Create | fsnotify.Write | fsnotify.Rename | fsnotify.Remove

func (w *fsnotifyWatcher) pump() {
	defer close(w.events)
	for {
		select {
		case ev, ok := <-w.fw.Events:
			if !ok {
				return
			}
			if ev.Op&watchOps == 0 {
				continue
			}
			select {
			case w.events <- ev.Name:
			default:
				// The consumer (watchLoop) isn't draining fast enough to
				// keep 64 slots free, which given its own loop does
				// nothing blocking should only happen under an
				// implausibly large simultaneous burst across many
				// paths. Dropping here trades a missed notification for
				// this path against blocking the whole OS notification
				// pump — a change to a path already in the pending set
				// is still recovered when that path's next event
				// arrives (or the batch settles on its already-changed
				// content); one this path has never been seen for isn't.
			}
		case _, ok := <-w.fw.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *fsnotifyWatcher) Events() <-chan string { return w.events }
func (w *fsnotifyWatcher) Close() error          { return w.fw.Close() }

// watchLoop pumps w.Events into a 200ms debounce and, once a burst
// settles, hops the accumulated set of changed paths onto the Store's
// own goroutine via Post to be handled under s.mu. It exits when either
// the Watcher's Events channel closes or watchStop is closed by Close.
func (s *fileStore) watchLoop() {
	defer close(s.watchStopped)

	var timer *time.Timer
	var timerC <-chan time.Time
	pending := map[string]bool{}

	for {
		select {
		case path, ok := <-s.watcher.Events():
			if !ok {
				return
			}
			pending[path] = true
			if timer == nil {
				timer = time.NewTimer(watchDebounce)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(watchDebounce)
			}
			timerC = timer.C

		case <-timerC:
			batch := pending
			pending = map[string]bool{}
			timerC = nil
			s.post(func() { s.handleWatchPaths(batch) })

		case <-s.watchStop:
			return
		}
	}
}

// handleWatchPaths runs on the Store's own goroutine (via Post) and
// applies the conflict policy (reload.go) to every path a debounce cycle
// settled on. Paths that no longer exist are processed before ones that
// do: an external rename (mv a.md b.md) surfaces as both a Remove-
// shaped and a Create-shaped event in the same batch, in no order the
// map's own iteration guarantees — handling a's disappearance first
// keeps parseIncomingProjectLocked from seeing a's id still registered
// when it parses b and minting a needless fresh identity for it.
func (s *fileStore) handleWatchPaths(paths map[string]bool) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}

	var gone, present []string
	for path := range paths {
		if _, err := s.fs.Stat(path); errors.Is(err, fs.ErrNotExist) {
			gone = append(gone, path)
		} else {
			present = append(present, path)
		}
	}

	var events []store.Event
	for _, path := range gone {
		events = append(events, s.handleWatchPathLocked(path)...)
	}
	for _, path := range present {
		events = append(events, s.handleWatchPathLocked(path)...)
	}
	s.mu.Unlock()

	s.emit(events...)
}
