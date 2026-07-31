package clipfmt

import (
	"strconv"
	"strings"

	"github.com/Yiin/ingot/internal/store"
)

// CopyAsList renders notes as a numbered Markdown ordered list — "1. ",
// "2. ", ... from 1, one entry per note in the order given, including a
// single note: this is "Copy as List", not "Copy as List if there are
// several". Each body's Markdown is kept verbatim (stripping it would
// silently damage a paste into a Markdown-aware chat box, the app's
// primary destination), and any continuation line within a body is
// indented by len(prefix) spaces — three for "1. ", four from "10. " —
// so the result stays a valid Markdown ordered list. There is no
// trailing newline. Done state and section titles are never included;
// callers pass only the note bodies they want listed.
func CopyAsList(notes []store.Note) string {
	entries := make([]string, len(notes))
	for i, n := range notes {
		prefix := strconv.Itoa(i+1) + ". "
		entries[i] = prefix + indentContinuation(n.Body, len(prefix))
	}
	return strings.Join(entries, "\n")
}

// indentContinuation indents every line of body after the first by
// indent spaces, so a multi-line body stays nested under its list
// marker instead of starting a new (unnumbered) line at column 0.
func indentContinuation(body string, indent int) string {
	lines := strings.Split(body, "\n")
	if len(lines) == 1 {
		return body
	}
	pad := strings.Repeat(" ", indent)
	for i := 1; i < len(lines); i++ {
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n")
}

// Copy renders notes as their raw bodies, one per note, joined with a
// blank line — no numbering, no prefix.
func Copy(notes []store.Note) string {
	bodies := make([]string, len(notes))
	for i, n := range notes {
		bodies[i] = n.Body
	}
	return strings.Join(bodies, "\n\n")
}
