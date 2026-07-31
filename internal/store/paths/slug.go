package paths

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// maxSlugBytes caps a slug's length. 64 bytes comfortably fits a
// filesystem's 255-byte name limit alongside "<slug>.md" and any
// "-<n>.<ext>" a caller like UniqueSlug or RotateBackup appends.
const maxSlugBytes = 64

// Slug turns an arbitrary project title into a filesystem-safe,
// human-readable identifier: NFC-normalize, lowercase, replace every rune
// that isn't a Unicode letter, digit, '-', or '_' with '-', collapse runs
// of '-', trim leading/trailing '-' and '_', then truncate to 64 bytes on
// a rune boundary. An empty result (an all-punctuation title, or one that
// normalizes away entirely) falls back to "project" so callers never see
// an empty slug. The title itself is never mutated — only this derived
// filename is.
func Slug(title string) string {
	lower := strings.ToLower(norm.NFC.String(title))

	var b strings.Builder
	b.Grow(len(lower))
	for _, r := range lower {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}

	slug := collapseDashes(b.String())
	slug = strings.Trim(slug, "-_")
	slug = truncateRunes(slug, maxSlugBytes)
	slug = strings.Trim(slug, "-_")

	if slug == "" {
		return "project"
	}
	return slug
}

// collapseDashes replaces every run of consecutive '-' with a single '-'.
func collapseDashes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range s {
		if r == '-' {
			if prevDash {
				continue
			}
			prevDash = true
		} else {
			prevDash = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// truncateRunes cuts s to at most maxBytes bytes, stopping before
// whichever rune would push it over the limit so a multi-byte rune is
// never split.
func truncateRunes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	total := 0
	for i, r := range s {
		if total+utf8.RuneLen(r) > maxBytes {
			return s[:i]
		}
		total += utf8.RuneLen(r)
	}
	return s
}

// UniqueSlug returns want if its case-folded form doesn't collide with
// any case-folded entry in existing, otherwise the first "want-2",
// "want-3", ... that doesn't.
func UniqueSlug(existing []string, want string) string {
	taken := make(map[string]bool, len(existing))
	for _, s := range existing {
		taken[strings.ToLower(s)] = true
	}
	if !taken[strings.ToLower(want)] {
		return want
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", want, n)
		if !taken[strings.ToLower(candidate)] {
			return candidate
		}
	}
}
