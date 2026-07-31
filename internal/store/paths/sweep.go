package paths

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yiin/ingot/internal/store/fsx"
)

// SweepTemp removes AtomicWrite temp files ("."+base+".tmp-"+rand) left
// in l.Projects by a process that died mid-write, as long as they're
// older than olderThan. A tmp file is never read by anything, so this is
// pure cleanup, not recovery: whichever of the old or new content made it
// to the live path is already the whole story.
func SweepTemp(fsys fsx.FS, l Layout, olderThan time.Duration) error {
	entries, err := fsys.ReadDir(l.Projects)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("paths: sweep temp: %w", err)
	}

	cutoff := time.Now().Add(-olderThan)
	for _, e := range entries {
		name := e.Name()
		if !isTempName(name) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return fmt.Errorf("paths: sweep temp: stat %s: %w", name, err)
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if err := fsys.Remove(filepath.Join(l.Projects, name)); err != nil {
			return fmt.Errorf("paths: sweep temp: remove %s: %w", name, err)
		}
	}
	return nil
}

// isTempName reports whether name is exactly the shape AtomicWrite's
// tmpName produces: ".<base>.tmp-<hex>". The suffix is anchored to the
// end of the string and required to be pure lowercase hex — not just
// "contains .tmp-" — so that a real project file can never be mistaken
// for a leftover temp file merely because ProjectFile validation once let
// a dot through (see validateSlug: it now rejects every dot, but this
// stays anchored regardless, since it's the cheaper, load-bearing check).
func isTempName(name string) bool {
	if !strings.HasPrefix(name, ".") {
		return false
	}
	i := strings.LastIndex(name, ".tmp-")
	if i < 0 {
		return false
	}
	suffix := name[i+len(".tmp-"):]
	if suffix == "" {
		return false
	}
	for _, r := range suffix {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
