package fsx

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"syscall"
)

// RotateBackup shifts live into backupDir/<base>.1, the previous .1 into
// .2, and so on up to .keep, dropping whatever falls off the end. Each
// shift is a Link (one inode, no data copy) unless the backup directory
// is on a different filesystem or the process lacks permission to link,
// in which case it falls back to a byte copy.
//
// It processes slots from keep down to 1 so that each source is read
// before a later step turns it into a destination.
func RotateBackup(fsys FS, live, backupDir string, keep int) error {
	if keep < 1 {
		return nil
	}
	if err := fsys.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("fsx: rotate backup: mkdir %s: %w", backupDir, err)
	}

	base := filepath.Base(live)
	slot := func(n int) string {
		return filepath.Join(backupDir, fmt.Sprintf("%s.%d", base, n))
	}

	for n := keep; n >= 1; n-- {
		src := live
		if n > 1 {
			src = slot(n - 1)
		}
		dst := slot(n)

		if _, err := fsys.Stat(src); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // nothing occupies this slot yet
			}
			return fmt.Errorf("fsx: rotate backup: stat %s: %w", src, err)
		}
		if err := fsys.Remove(dst); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("fsx: rotate backup: remove %s: %w", dst, err)
		}
		if err := linkOrCopy(fsys, src, dst); err != nil {
			return fmt.Errorf("fsx: rotate backup: %s -> %s: %w", src, dst, err)
		}
	}

	// A backup surviving a crash is best-effort, not the same crash
	// guarantee AtomicWrite gives the live file — but fsyncing the
	// directory once, after every slot has landed, costs one syscall and
	// closes the gap where a rotated-in backup is visible but not
	// durable.
	if err := fsys.SyncDir(backupDir); err != nil {
		return fmt.Errorf("fsx: rotate backup: sync dir %s: %w", backupDir, err)
	}
	return nil
}

func linkOrCopy(fsys FS, src, dst string) error {
	err := fsys.Link(src, dst)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) && !errors.Is(err, syscall.EPERM) {
		return err
	}

	data, err := fsys.ReadFile(src)
	if err != nil {
		return err
	}
	f, err := fsys.Create(dst)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
