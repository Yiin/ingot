package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// invocationRE finds an `ingot <word>` invocation in prose or in a fenced
// code block. The word is captured only when it looks like a subcommand:
// lowercase letters and dashes, so `ingot --hidden`, `ingot <project>` and
// a bare `ingot` are all left alone. Trailing punctuation is excluded by
// the character class rather than stripped afterwards.
var invocationRE = regexp.MustCompile("`?\\bingot ([a-z][a-z-]*)")

// docFiles are every file that tells a user what to type. Each one is a
// place the CLI's real command set can silently drift away from.
var docFiles = []string{
	"../../README.md",
	"../../contrib/ingot.desktop",
	"../../contrib/ingot.service",
}

// TestDocumentedCommandsExist fails when any doc names an `ingot <cmd>`
// that Run would reject.
//
// This is the exact bug it was written for: the README told users to bind
// their compositor to `ingot toggle`, which was never a subcommand, so the
// dispatch fell through to usage and exit 2 and the keybind did nothing.
// Nothing connected the prose to the dispatch table, so the mistake
// survived every test in the repo.
func TestDocumentedCommandsExist(t *testing.T) {
	for _, path := range docFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		for _, m := range invocationRE.FindAllStringSubmatch(string(raw), -1) {
			cmd := m[1]
			if _, ok := subcommands[cmd]; !ok {
				t.Errorf("%s documents `ingot %s`, which is not a subcommand — Run prints usage and exits 2 for it",
					filepath.Base(path), cmd)
			}
		}
	}
}

// TestUsageDocumentsEveryCommand is the other direction: a subcommand
// that never appears in the usage text is undiscoverable, since usage is
// the only thing `ingot` prints when it does not understand its
// arguments.
func TestUsageDocumentsEveryCommand(t *testing.T) {
	for cmd := range subcommands {
		// "run" is the implicit default a bare `ingot` dispatches to, and
		// usage lists both spellings, so the plain substring below is a
		// sound check for it too.
		if !strings.Contains(usage, "ingot "+cmd) {
			t.Errorf("subcommand %q is not documented in the usage text", cmd)
		}
	}
}

// TestUsageNamesOnlyRealCommands stops the usage text drifting the way
// the README did. Every `ingot <word>` it prints must dispatch.
func TestUsageNamesOnlyRealCommands(t *testing.T) {
	for _, m := range invocationRE.FindAllStringSubmatch(usage, -1) {
		if _, ok := subcommands[m[1]]; !ok {
			t.Errorf("usage documents `ingot %s`, which is not a subcommand", m[1])
		}
	}
}
