package searchtext

import "testing"

func TestTokensFoldsAndSplits(t *testing.T) {
	got := Tokens("  TOML   Café  ")
	want := []string{"toml", "cafe"}
	if len(got) != len(want) {
		t.Fatalf("Tokens = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Tokens[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTokensEmptyQuery(t *testing.T) {
	if got := Tokens("   "); len(got) != 0 {
		t.Errorf("Tokens(whitespace) = %v, want empty", got)
	}
}

func TestMatchRequiresEveryToken(t *testing.T) {
	n := Normalize("buy milk and eggs")
	if matched, _ := n.Match(Tokens("buy eggs")); !matched {
		t.Error("Match(buy eggs) = false, want true (AND, order independent)")
	}
	if matched, _ := n.Match(Tokens("eggs buy")); !matched {
		t.Error("Match(eggs buy) = false, want true (order independent)")
	}
	if matched, _ := n.Match(Tokens("buy bread")); matched {
		t.Error("Match(buy bread) = true, want false (bread absent)")
	}
}

func TestMatchNoTokensNeverMatches(t *testing.T) {
	n := Normalize("anything")
	if matched, ranges := n.Match(nil); matched || ranges != nil {
		t.Errorf("Match(nil) = (%v, %v), want (false, nil)", matched, ranges)
	}
}

// TestMatchEmptyStringTokenDoesNotPanic guards against a caller passing
// Match a literal "" token directly (Tokens itself never produces one,
// but Match is exported): strings.Contains(x, "") is unconditionally
// true, so a naive implementation indexes occurrences at a zero-length
// match and reads one before the start of the offset table.
func TestMatchEmptyStringTokenDoesNotPanic(t *testing.T) {
	n := Normalize("anything")
	if matched, ranges := n.Match([]string{""}); matched || ranges != nil {
		t.Errorf(`Match([""]) = (%v, %v), want (false, nil)`, matched, ranges)
	}
	if matched, _ := n.Match([]string{"", "anything"}); !matched {
		t.Error(`Match(["", "anything"]) = false, want true (empty token ignored, not treated as a failing token)`)
	}
}

// TestMatchInvalidUTF8DoesNotProduceOutOfBoundsRange guards against
// ranging over invalid UTF-8: Go's range-over-string turns any invalid
// byte into U+FFFD, and utf8.RuneLen(U+FFFD) is 3 even though the
// invalid byte it replaced was 1 raw byte — using RuneLen to size the
// raw span would walk starts/ends past len(body).
func TestMatchInvalidUTF8DoesNotProduceOutOfBoundsRange(t *testing.T) {
	body := "abc\xff"
	n := Normalize(body)
	matched, ranges := n.Match(Tokens(body))
	if !matched {
		t.Fatalf("Match against its own body = false, want true")
	}
	for _, r := range ranges {
		if r[0] < 0 || r[1] > len(body) || r[0] > r[1] {
			t.Fatalf("range %v out of bounds for a %d-byte body", r, len(body))
		}
		_ = body[r[0]:r[1]] // must not panic
	}
}

// TestMatchTOMLDefaultAcceptanceCriterion reproduces the exact example
// from the child issue's acceptance criteria: querying "toml default"
// finds a hit inside "Use **TOML as the default declarative format**"
// even though "TOML" and "default" both sit next to Markdown emphasis
// markers.
func TestMatchTOMLDefaultAcceptanceCriterion(t *testing.T) {
	body := "Use **TOML as the default declarative format**"
	n := Normalize(body)
	matched, ranges := n.Match(Tokens("toml default"))
	if !matched {
		t.Fatalf("Match(toml default) against %q = false, want true", body)
	}
	if len(ranges) != 2 {
		t.Fatalf("ranges = %v, want 2 entries", ranges)
	}
	tomlStart := indexOf(t, body, "TOML")
	defaultStart := indexOf(t, body, "default")
	wantToml := [2]int{tomlStart, tomlStart + len("TOML")}
	wantDefault := [2]int{defaultStart, defaultStart + len("default")}
	if ranges[0] != wantToml {
		t.Errorf("ranges[0] = %v, want %v (TOML)", ranges[0], wantToml)
	}
	if ranges[1] != wantDefault {
		t.Errorf("ranges[1] = %v, want %v (default)", ranges[1], wantDefault)
	}
}

// TestMatchMarkerTokenCreatesNoFalseHit is the acceptance criterion's
// negative case: a token that only exists as Markdown syntax (the "**"
// marker itself, stripped before matching) must not create a hit.
func TestMatchMarkerTokenCreatesNoFalseHit(t *testing.T) {
	body := "Use **TOML** now"
	n := Normalize(body)
	if matched, _ := n.Match(Tokens("**")); matched {
		t.Error(`Match("**") = true, want false: "**" is stripped Markdown syntax, not content`)
	}
}

func TestMatchRangesPointAtRawBodyBytes(t *testing.T) {
	body := "Use **TOML** now"
	n := Normalize(body)
	matched, ranges := n.Match(Tokens("toml"))
	if !matched {
		t.Fatalf("Match(toml) = false, want true")
	}
	start := indexOf(t, body, "TOML")
	want := [2]int{start, start + len("TOML")}
	if len(ranges) != 1 || ranges[0] != want {
		t.Errorf("ranges = %v, want [%v]", ranges, want)
	}
	if got := body[ranges[0][0]:ranges[0][1]]; got != "TOML" {
		t.Errorf("body[range] = %q, want %q", got, "TOML")
	}
}

func TestMatchLinkURLIsNotSearchable(t *testing.T) {
	body := "See [docs](https://toml.io/spec) for the grammar"
	n := Normalize(body)
	if matched, _ := n.Match(Tokens("toml")); matched {
		t.Error("Match(toml) = true, want false: token only appears in the stripped link URL")
	}
	if matched, _ := n.Match(Tokens("docs")); !matched {
		t.Error("Match(docs) = false, want true: token is in the link text")
	}
}

func indexOf(t *testing.T, s, substr string) int {
	t.Helper()
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	t.Fatalf("%q not found in %q", substr, s)
	return -1
}
