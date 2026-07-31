package paths

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ProjectFile returns the Markdown path for slug under l.Projects. It
// re-checks that slug is a single, safe path component — no separator, no
// dot at all, not empty — independently of Slug ever having run, so a
// slug read back from an untrusted source (a crafted meta sidecar, a
// hand-edited file) can never join its way outside Projects, and can
// never collide with the "."-prefixed, ".tmp-"-containing names
// SweepTemp treats as safe to delete on sight. Rejecting every dot, not
// just "." and "..", costs nothing: Slug's own output never contains one
// — dots are always replaced by '-' — so no slug Slug produces is ever
// turned away here.
func ProjectFile(l Layout, slug string) (string, error) {
	if err := validateSlug(slug); err != nil {
		return "", err
	}
	return filepath.Join(l.Projects, slug+".md"), nil
}

// MetaFile returns the provenance sidecar path for slug under l.Meta,
// mirroring ProjectFile's validation so a slug read back from an
// untrusted source can never escape Meta either.
func MetaFile(l Layout, slug string) (string, error) {
	if err := validateSlug(slug); err != nil {
		return "", err
	}
	return filepath.Join(l.Meta, slug+".json"), nil
}

func validateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("paths: invalid slug %q: empty", slug)
	}
	if strings.ContainsAny(slug, "/\\") {
		return fmt.Errorf("paths: invalid slug %q: contains a path separator", slug)
	}
	if strings.Contains(slug, ".") {
		return fmt.Errorf("paths: invalid slug %q: contains a dot", slug)
	}
	return nil
}
