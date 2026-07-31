package paths

import "testing"

func TestSlug(t *testing.T) {
	// nfdCafe is "cafe" with a combining acute accent (U+0301) after the
	// e, i.e. NFD form. The accent alone is a combining mark, not a
	// letter, so without NFC normalization first it would be replaced by
	// a dash and trimmed away, losing the accent entirely.
	nfdCafe := "cafe\u0301" // caf + e + combining acute accent
	nfcCafe := "caf\u00e9"  // caf + precomposed e-acute

	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"simple words with separators", "Home / Garden", "home-garden"},
		{"emoji prefix collapses away", "\U0001F680 Launch", "launch"},
		{"all dots falls back to project", "..", "project"},
		{"already lowercase with dash", "my-notes", "my-notes"},
		{"underscore preserved", "my_notes", "my_notes"},
		{"leading and trailing punctuation trimmed", "!!!Ideas!!!", "ideas"},
		{"all punctuation falls back to project", "!!!___...", "project"},
		{"empty title falls back to project", "", "project"},
		{"CJK survives as letters", "日本語ノート", "日本語ノート"},
		{"mixed CJK and latin", "Recipes 料理", "recipes-料理"},
		{"NFD input normalizes before slugging", nfdCafe, nfcCafe},
		{"path traversal is neutralized", "A/../../etc/passwd", "a-etc-passwd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Slug(tt.title); got != tt.want {
				t.Errorf("Slug(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestSlug_Truncates300CharsToRuneBoundary(t *testing.T) {
	title := ""
	for i := 0; i < 300; i++ {
		title += "a"
	}
	got := Slug(title)
	if len(got) > maxSlugBytes {
		t.Fatalf("Slug(300 chars) len = %d, want <= %d", len(got), maxSlugBytes)
	}
	if len(got) != maxSlugBytes {
		t.Errorf("Slug(300 chars) len = %d, want exactly %d", len(got), maxSlugBytes)
	}
	for _, r := range got {
		if r != 'a' {
			t.Fatalf("Slug(300 chars) = %q, want all 'a'", got)
		}
	}
}

func TestSlug_TruncatesMultiByteRunesOnBoundary(t *testing.T) {
	// Each U+65E5 is 3 bytes; 30 of them is 90 bytes, well past the
	// 64-byte cap, and 64 is not a multiple of 3 so a naive byte-slice
	// truncation would split the rune straddling the boundary.
	title := ""
	for i := 0; i < 30; i++ {
		title += "日"
	}
	got := Slug(title)
	if len(got) > maxSlugBytes {
		t.Fatalf("Slug len = %d, want <= %d", len(got), maxSlugBytes)
	}
	for i, r := range got {
		if r == 0xFFFD {
			t.Fatalf("Slug(%q)[%d] is an invalid rune — truncation split a multi-byte character", got, i)
		}
	}
}

func TestUniqueSlug(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		want     string
		expect   string
	}{
		{"no collision returns want unchanged", []string{"foo", "bar"}, "baz", "baz"},
		{"empty existing returns want unchanged", nil, "baz", "baz"},
		{"first collision appends -2", []string{"foo"}, "foo", "foo-2"},
		{"case-folded collision still collides", []string{"Foo"}, "foo", "foo-2"},
		{"skips taken suffixes", []string{"foo", "foo-2", "foo-3"}, "foo", "foo-4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UniqueSlug(tt.existing, tt.want); got != tt.expect {
				t.Errorf("UniqueSlug(%v, %q) = %q, want %q", tt.existing, tt.want, got, tt.expect)
			}
		})
	}
}
