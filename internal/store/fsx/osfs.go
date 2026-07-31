package fsx

import (
	"io/fs"
	"os"
)

// osFS is the real, disk-backed FS.
type osFS struct{}

// OS returns the FS backed by the actual filesystem.
func OS() FS { return osFS{} }

func (osFS) ReadDir(path string) ([]fs.DirEntry, error) { return os.ReadDir(path) }

func (osFS) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (osFS) Create(path string) (File, error) {
	// O_EXCL: a temp file name colliding with a live write is a bug we
	// want to surface, not silently clobber.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (osFS) Rename(oldpath, newpath string) error { return os.Rename(oldpath, newpath) }

func (osFS) Remove(path string) error { return os.Remove(path) }

func (osFS) Link(oldpath, newpath string) error { return os.Link(oldpath, newpath) }

func (osFS) Stat(path string) (fs.FileInfo, error) { return os.Stat(path) }

func (osFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }

func (osFS) SyncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}
