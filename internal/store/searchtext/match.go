package searchtext

import (
	"sort"
	"strings"
)

// Tokens splits query on whitespace into match tokens, folded the same
// way a Normalize'd body's literal text is (NFKD, combining marks
// stripped, lower-cased) — but never Markdown-stripped, since a query
// is plain text a person typed, not Markdown source.
func Tokens(query string) []string {
	fields := strings.Fields(query)
	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		if folded := foldPlain(f); folded != "" {
			tokens = append(tokens, folded)
		}
	}
	return tokens
}

// Match reports whether every (non-empty) token in tokens is a
// substring of n — case-, diacritic-, and Markdown-insensitive
// AND-substring matching, order-independent across tokens. When
// matched, ranges gives every occurrence of every token as a raw-body
// [start, end) byte pair, sorted by start. A nil, empty, or
// all-empty-strings tokens never matches — Tokens never produces an
// empty string, but Match is exported and does not assume its caller
// went through Tokens first.
func (n Normalized) Match(tokens []string) (matched bool, ranges [][2]int) {
	any := false
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		any = true
		if !strings.Contains(n.Text, tok) {
			return false, nil
		}
	}
	if !any {
		return false, nil
	}
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		ranges = append(ranges, n.occurrences(tok)...)
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i][0] != ranges[j][0] {
			return ranges[i][0] < ranges[j][0]
		}
		return ranges[i][1] < ranges[j][1]
	})
	return true, ranges
}

// occurrences returns every non-overlapping occurrence of tok in n.Text
// as raw-body [start, end) byte pairs. tok must be non-empty.
func (n Normalized) occurrences(tok string) [][2]int {
	var out [][2]int
	pos := 0
	for {
		idx := strings.Index(n.Text[pos:], tok)
		if idx < 0 {
			return out
		}
		start := pos + idx
		end := start + len(tok)
		out = append(out, [2]int{n.starts[start], n.ends[end-1]})
		pos = end
	}
}
