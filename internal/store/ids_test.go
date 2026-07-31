package store

import "testing"

func TestNewID(t *testing.T) {
	const draws = 100_000
	seen := make(map[string]bool, draws)
	for i := 0; i < draws; i++ {
		id := NewID()
		if len(id) != 16 {
			t.Fatalf("NewID() = %q, want length 16", id)
		}
		for _, c := range id {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Fatalf("NewID() = %q, contains non-lowercase-hex character %q", id, c)
			}
		}
		if seen[id] {
			t.Fatalf("NewID() collided at draw %d: %q", i, id)
		}
		seen[id] = true
	}
}
