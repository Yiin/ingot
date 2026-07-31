package panel

import "testing"

// TestDecideState covers RefreshEmptyState's decision table without a
// GTK display: search-no-matches wins over the first-run hint whenever a
// non-empty query is producing zero results, even in an otherwise
// non-empty project.
func TestDecideState(t *testing.T) {
	tests := []struct {
		name       string
		hasNotes   bool
		query      string
		matchCount int
		want       state
	}{
		{"first run, no notes, no query", false, "", 0, stateHint},
		{"notes exist, no query", true, "", 0, stateNormal},
		{"notes exist, query with matches", true, "shift", 2, stateNormal},
		{"notes exist, query with no matches", true, "xyzzy", 0, stateSearchEmpty},
		{"no notes at all, actively searching", false, "xyzzy", 0, stateSearchEmpty},
		{"empty query with a stale zero count", true, "", 0, stateNormal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decideState(tt.hasNotes, tt.query, tt.matchCount); got != tt.want {
				t.Errorf("decideState(%v, %q, %d) = %v, want %v", tt.hasNotes, tt.query, tt.matchCount, got, tt.want)
			}
		})
	}
}

func TestSearchEmptyText(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"shift", `No notes match "shift"`},
		{`quote"inside`, `No notes match "quote\"inside"`},
	}
	for _, tt := range tests {
		if got := searchEmptyText(tt.query); got != tt.want {
			t.Errorf("searchEmptyText(%q) = %q, want %q", tt.query, got, tt.want)
		}
	}
}
