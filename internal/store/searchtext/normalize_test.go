package searchtext

import "testing"

func TestNormalizeStripsMarkdown(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"bold", "Use **TOML** now", "use toml now"},
		{"italic underscore", "an _emphasis_ word", "an emphasis word"},
		{"code span", "run `go test` please", "run go test please"},
		{"link keeps text drops url", "[TOML docs](https://example.com/x)", "toml docs"},
		{"stray asterisk stays literal", "5 * 3 * 2 = 30", "5 * 3 * 2 = 30"},
		{"snake case stays literal", "snake_case_name", "snake_case_name"},
		{"heading", "# Title", "title"},
		{"plain", "buy milk and eggs", "buy milk and eggs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.body).Text
			if got != tt.want {
				t.Errorf("Normalize(%q).Text = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestNormalizeDiacriticInsensitive(t *testing.T) {
	got := Normalize("Café résumé naïve").Text
	want := "cafe resume naive"
	if got != want {
		t.Errorf("Normalize(...).Text = %q, want %q", got, want)
	}
}

func TestNormalizeOffsetsMapRuneBoundaries(t *testing.T) {
	// "café" — é is 2 raw bytes (U+00E9), but folds to 1 byte ("e").
	n := Normalize("café")
	if n.Text != "cafe" {
		t.Fatalf("Text = %q, want %q", n.Text, "cafe")
	}
	matched, ranges := n.Match([]string{"cafe"})
	if !matched {
		t.Fatalf("Match(cafe) = false, want true")
	}
	if len(ranges) != 1 {
		t.Fatalf("ranges = %v, want 1 entry", ranges)
	}
	// raw "café": c(0) a(1) f(2) é(3..5) — full raw span is [0,5).
	if got, want := ranges[0], ([2]int{0, 5}); got != want {
		t.Errorf("range = %v, want %v", got, want)
	}
}

func TestNormalizeEmptyBody(t *testing.T) {
	n := Normalize("")
	if n.Text != "" {
		t.Errorf("Text = %q, want empty", n.Text)
	}
	if matched, _ := n.Match([]string{"x"}); matched {
		t.Error("Match on empty body = true, want false")
	}
}

func TestNormalizeMultiParagraphSeparatesWithSpace(t *testing.T) {
	got := Normalize("first\n\nsecond").Text
	want := "first second"
	if got != want {
		t.Errorf("Text = %q, want %q", got, want)
	}
}
