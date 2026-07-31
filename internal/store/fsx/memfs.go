package fsx

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// failRule is one fault-injection rule: the next call matching op and the
// path pattern returns err instead of touching the in-memory state. A
// pattern ending in "*" matches by prefix, which is what lets a test
// target AtomicWrite's randomly-suffixed temp file without knowing the
// suffix ahead of time.
type failRule struct {
	op      string
	pattern string
	err     error
}

// MemFS is an in-memory FS for tests: no real disk, deterministic fault
// injection per operation, and a hook to observe state after every call.
type MemFS struct {
	mu       sync.Mutex
	files    map[string][]byte
	modTimes map[string]time.Time
	dirs     map[string]bool
	failures []failRule
	observe  func(op, path string)
}

// NewMem returns an empty MemFS with the root directory "/" pre-created.
func NewMem() *MemFS {
	return &MemFS{
		files:    make(map[string][]byte),
		modTimes: make(map[string]time.Time),
		dirs:     map[string]bool{"/": true, ".": true},
	}
}

// FailOn makes every future call to op whose path matches pattern return
// err instead of running. pattern is an exact path, or a prefix match if
// it ends in "*".
func (m *MemFS) FailOn(op, pattern string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures = append(m.failures, failRule{op: op, pattern: pattern, err: err})
}

// SetObserver installs a hook called after every operation that wasn't
// itself fault-injected, so a test can assert on filesystem state at
// each step of a multi-syscall sequence like AtomicWrite. A call that
// FailOn intercepts returns before this hook runs — it never fires for
// that call, since there's no new state for it to observe.
func (m *MemFS) SetObserver(fn func(op, path string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observe = fn
}

func matchPattern(pattern, path string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(path, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == path
}

// checkFail must be called without m.mu held; it locks internally.
func (m *MemFS) checkFail(op, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.failures {
		if r.op == op && matchPattern(r.pattern, path) {
			return r.err
		}
	}
	return nil
}

func (m *MemFS) notify(op, path string) {
	m.mu.Lock()
	fn := m.observe
	m.mu.Unlock()
	if fn != nil {
		fn(op, path)
	}
}

func notExist(path string) error {
	return &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
}

func (m *MemFS) ReadDir(dir string) ([]fs.DirEntry, error) {
	if err := m.checkFail("ReadDir", dir); err != nil {
		return nil, err
	}
	defer m.notify("ReadDir", dir)

	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.dirs[dir] {
		return nil, notExist(dir)
	}
	seen := make(map[string]bool)
	var entries []fs.DirEntry
	for p := range m.files {
		if filepath.Dir(p) != dir {
			continue
		}
		name := filepath.Base(p)
		if seen[name] {
			continue
		}
		seen[name] = true
		entries = append(entries, memDirEntry{info: m.infoLocked(p, name)})
	}
	for p := range m.dirs {
		if p == dir || filepath.Dir(p) != dir {
			continue
		}
		name := filepath.Base(p)
		if seen[name] {
			continue
		}
		seen[name] = true
		entries = append(entries, memDirEntry{info: memFileInfo{name: name, isDir: true}})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

func (m *MemFS) ReadFile(path string) ([]byte, error) {
	if err := m.checkFail("ReadFile", path); err != nil {
		return nil, err
	}
	defer m.notify("ReadFile", path)

	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[path]
	if !ok {
		return nil, notExist(path)
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

// Create matches osFS's O_EXCL semantics: it fails if path already
// exists rather than silently truncating it, since a real collision (an
// AtomicWrite temp name reused, or any code path that Creates a live
// path outright) is a bug worth surfacing in both implementations
// identically, not one MemFS papers over.
func (m *MemFS) Create(path string) (File, error) {
	if err := m.checkFail("Create", path); err != nil {
		return nil, err
	}
	defer m.notify("Create", path)

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.files[path]; exists {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrExist}
	}
	dir := filepath.Dir(path)
	if !m.dirs[dir] {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}
	m.files[path] = nil
	m.modTimes[path] = time.Now()
	return &memHandle{fsys: m, path: path}, nil
}

func (m *MemFS) Rename(oldpath, newpath string) error {
	if err := m.checkFail("Rename", oldpath); err != nil {
		return err
	}
	if err := m.checkFail("Rename", newpath); err != nil {
		return err
	}
	defer m.notify("Rename", newpath)

	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[oldpath]
	if !ok {
		return notExist(oldpath)
	}
	dir := filepath.Dir(newpath)
	if !m.dirs[dir] {
		return &fs.PathError{Op: "rename", Path: newpath, Err: fs.ErrNotExist}
	}
	m.files[newpath] = data
	m.modTimes[newpath] = time.Now()
	delete(m.files, oldpath)
	delete(m.modTimes, oldpath)
	return nil
}

func (m *MemFS) Remove(path string) error {
	if err := m.checkFail("Remove", path); err != nil {
		return err
	}
	defer m.notify("Remove", path)

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[path]; ok {
		delete(m.files, path)
		delete(m.modTimes, path)
		return nil
	}
	if m.dirs[path] {
		if m.hasChildrenLocked(path) {
			return &fs.PathError{Op: "remove", Path: path, Err: syscall.ENOTEMPTY}
		}
		delete(m.dirs, path)
		return nil
	}
	return notExist(path)
}

func (m *MemFS) Link(oldpath, newpath string) error {
	if err := m.checkFail("Link", oldpath); err != nil {
		return err
	}
	if err := m.checkFail("Link", newpath); err != nil {
		return err
	}
	defer m.notify("Link", newpath)

	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[oldpath]
	if !ok {
		return notExist(oldpath)
	}
	dir := filepath.Dir(newpath)
	if !m.dirs[dir] {
		return &fs.PathError{Op: "link", Path: newpath, Err: fs.ErrNotExist}
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	m.files[newpath] = cp
	m.modTimes[newpath] = time.Now()
	return nil
}

func (m *MemFS) Stat(path string) (fs.FileInfo, error) {
	if err := m.checkFail("Stat", path); err != nil {
		return nil, err
	}
	defer m.notify("Stat", path)

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[path]; ok {
		return m.infoLocked(path, filepath.Base(path)), nil
	}
	if m.dirs[path] {
		return memFileInfo{name: filepath.Base(path), isDir: true}, nil
	}
	return nil, notExist(path)
}

// hasChildrenLocked reports whether any file or directory is directly
// inside path. Callers must hold m.mu.
func (m *MemFS) hasChildrenLocked(path string) bool {
	for p := range m.files {
		if filepath.Dir(p) == path {
			return true
		}
	}
	for p := range m.dirs {
		if p != path && filepath.Dir(p) == path {
			return true
		}
	}
	return false
}

func (m *MemFS) infoLocked(path, name string) memFileInfo {
	return memFileInfo{
		name:    name,
		size:    int64(len(m.files[path])),
		modTime: m.modTimes[path],
	}
}

func (m *MemFS) MkdirAll(path string, _ os.FileMode) error {
	if err := m.checkFail("MkdirAll", path); err != nil {
		return err
	}
	defer m.notify("MkdirAll", path)

	m.mu.Lock()
	defer m.mu.Unlock()
	p := path
	for {
		m.dirs[p] = true
		parent := filepath.Dir(p)
		if parent == p || parent == "." || parent == "/" {
			m.dirs[parent] = true
			break
		}
		p = parent
	}
	return nil
}

func (m *MemFS) SyncDir(path string) error {
	if err := m.checkFail("SyncDir", path); err != nil {
		return err
	}
	defer m.notify("SyncDir", path)

	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.dirs[path] {
		return notExist(path)
	}
	return nil
}

// memHandle is the File returned by MemFS.Create. Writes land immediately
// in the backing map, mirroring how a real file's content is visible to
// any fd on it before fsync, while Sync/Close still go through fault
// injection so tests can fail either step independently.
//
// Its internal buffer is unguarded: like an *os.File, a single handle is
// not safe for concurrent use from multiple goroutines. Every caller in
// this codebase (AtomicWrite, RotateBackup's copy fallback) writes from
// one goroutine only.
type memHandle struct {
	fsys *MemFS
	path string
	buf  bytes.Buffer
}

func (h *memHandle) Write(p []byte) (int, error) {
	if err := h.fsys.checkFail("Write", h.path); err != nil {
		return 0, err
	}
	n, _ := h.buf.Write(p)
	h.fsys.mu.Lock()
	h.fsys.files[h.path] = append([]byte(nil), h.buf.Bytes()...)
	h.fsys.modTimes[h.path] = time.Now()
	h.fsys.mu.Unlock()
	h.fsys.notify("Write", h.path)
	return n, nil
}

func (h *memHandle) Sync() error {
	if err := h.fsys.checkFail("Sync", h.path); err != nil {
		return err
	}
	h.fsys.notify("Sync", h.path)
	return nil
}

func (h *memHandle) Close() error {
	if err := h.fsys.checkFail("Close", h.path); err != nil {
		return err
	}
	h.fsys.notify("Close", h.path)
	return nil
}

type memFileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

func (i memFileInfo) Name() string       { return i.name }
func (i memFileInfo) Size() int64        { return i.size }
func (i memFileInfo) ModTime() time.Time { return i.modTime }
func (i memFileInfo) IsDir() bool        { return i.isDir }
func (i memFileInfo) Sys() any           { return nil }
func (i memFileInfo) Mode() fs.FileMode {
	if i.isDir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

type memDirEntry struct{ info memFileInfo }

func (e memDirEntry) Name() string               { return e.info.name }
func (e memDirEntry) IsDir() bool                { return e.info.isDir }
func (e memDirEntry) Type() fs.FileMode          { return e.info.Mode().Type() }
func (e memDirEntry) Info() (fs.FileInfo, error) { return e.info, nil }
