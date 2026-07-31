package fsx

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
)

// AtomicWrite replaces path with data such that, at every instant, a
// reader sees either the exact old bytes or the exact new bytes — never a
// partial write. It writes to a temp file in the same directory (so the
// rename is same-filesystem and therefore atomic), fsyncs it, renames it
// over path, then fsyncs the directory so the rename itself survives a
// crash. No journal or lock file is needed because of that ordering.
func AtomicWrite(fsys FS, path string, data []byte) error {
	dir := filepath.Dir(path)
	tmpPath := filepath.Join(dir, tmpName(filepath.Base(path)))

	f, err := fsys.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("fsx: atomic write %s: create temp: %w", path, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = fsys.Remove(tmpPath)
		return fmt.Errorf("fsx: atomic write %s: write temp: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = fsys.Remove(tmpPath)
		return fmt.Errorf("fsx: atomic write %s: sync temp: %w", path, err)
	}
	if err := f.Close(); err != nil {
		_ = fsys.Remove(tmpPath)
		return fmt.Errorf("fsx: atomic write %s: close temp: %w", path, err)
	}
	if err := fsys.Rename(tmpPath, path); err != nil {
		_ = fsys.Remove(tmpPath)
		return fmt.Errorf("fsx: atomic write %s: rename: %w", path, err)
	}
	if err := fsys.SyncDir(dir); err != nil {
		return fmt.Errorf("fsx: atomic write %s: sync dir: %w", path, err)
	}
	return nil
}

// tmpName follows the ".<base>.tmp-<rand>" convention paths.SweepTemp
// recognizes as safe to delete on startup — a tmp file is never read.
func tmpName(base string) string {
	return fmt.Sprintf(".%s.tmp-%x", base, rand.Uint64())
}
