package fsx

import (
	"io"
	"io/fs"
	"os"
)

// File is a handle returned by Create. Sync is separated from Close so
// callers can force data to stable storage before releasing the handle,
// which AtomicWrite relies on.
type File interface {
	io.Writer
	io.Closer
	Sync() error
}

// FS is the filesystem seam the whole store is built on. Every method
// mirrors a single real syscall, which keeps OS() a thin wrapper and
// keeps NewMem() able to fault-inject each one independently.
type FS interface {
	ReadDir(path string) ([]fs.DirEntry, error)
	ReadFile(path string) ([]byte, error)
	Create(path string) (File, error)
	Rename(oldpath, newpath string) error
	Remove(path string) error
	Link(oldpath, newpath string) error
	Stat(path string) (fs.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
	// SyncDir fsyncs the directory at path, so a prior Rename or Create
	// within it is durable, not just visible.
	SyncDir(path string) error
}
