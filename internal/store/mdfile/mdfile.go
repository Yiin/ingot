package mdfile

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Yiin/ingot/internal/store"
)

// CurrentSchema is the front-matter schema version this package reads
// and writes. Format refuses to write a Project whose Schema exceeds
// it, so an older binary can never corrupt a file a newer one wrote.
const CurrentSchema = 1

// ErrSchemaTooNew is returned by Format when the Project's Schema is
// greater than CurrentSchema.
var ErrSchemaTooNew = errors.New("mdfile: schema is newer than this binary supports")

// Warning describes a recoverable oddity found while parsing — a
// malformed front-matter line, an unterminated front-matter block, or a
// known key whose value doesn't fit its expected shape. Parse never
// fails because of one; it does its best and reports what it noticed.
type Warning struct {
	// Line is the 1-indexed input line the warning refers to.
	Line int
	// Message describes the problem, in prose.
	Message string
}

func (w Warning) String() string {
	return fmt.Sprintf("line %d: %s", w.Line, w.Message)
}

// Parse reads a project file's raw bytes into a store.Project. The
// grammar is total: every byte sequence produces some Project, since
// any non-blank line that isn't front matter or a "## " heading is
// promoted to a note verbatim. Section and note ids are minted fresh
// (they are never persisted); the project id, title, and created time
// come from front matter and are empty/zero when front matter is
// absent, for the caller to fill in.
func Parse(b []byte) (store.Project, []Warning, error) {
	content := strings.ReplaceAll(string(b), "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := strings.Split(content, "\n")

	var warnings []Warning
	proj := store.Project{Schema: CurrentSchema}
	extra := map[string]string{}

	start := 0
	if len(lines) > 0 && lines[0] == "---" {
		end := -1
		for j := 1; j < len(lines); j++ {
			if lines[j] == "---" {
				end = j
				break
			}
		}
		if end == -1 {
			warnings = append(warnings, Warning{
				Line:    1,
				Message: `unterminated front matter (no closing "---"); treated as content`,
			})
		} else {
			for j := 1; j < end; j++ {
				line := lines[j]
				if strings.TrimSpace(line) == "" {
					continue
				}
				idx := strings.Index(line, ":")
				if idx == -1 {
					warnings = append(warnings, Warning{
						Line:    j + 1,
						Message: "malformed front-matter line (missing ':'): " + line,
					})
					continue
				}
				key := line[:idx]
				val := strings.TrimSpace(line[idx+1:])
				switch key {
				case "ingot":
					n, err := strconv.Atoi(val)
					if err != nil {
						warnings = append(warnings, Warning{
							Line:    j + 1,
							Message: "invalid ingot schema value: " + val,
						})
					} else {
						proj.Schema = n
					}
				case "id":
					if isValidID(val) {
						proj.ID = store.ProjectID(val)
					} else {
						warnings = append(warnings, Warning{
							Line:    j + 1,
							Message: "invalid id value: " + val,
						})
						extra["id"] = val
					}
				case "title":
					proj.Title = val
				case "created":
					t, err := time.Parse(time.RFC3339, val)
					if err != nil {
						warnings = append(warnings, Warning{
							Line:    j + 1,
							Message: "invalid created timestamp: " + val,
						})
						extra["created"] = val
					} else {
						proj.Created = t
					}
				default:
					extra[key] = val
				}
			}
			start = end + 1
		}
	}

	sections := parseBody(lines[start:])
	proj.Sections = sections
	if len(extra) > 0 {
		proj.Extra = extra
	}
	return proj, warnings, nil
}

// parseBody parses everything after any front matter: section headings
// and notes.
func parseBody(lines []string) []store.Section {
	var sections []store.Section
	curTitle := ""
	var curNotes []store.Note

	haveNote := false
	var noteLines []string
	noteDone := false
	var pendingBlanks []string

	commitNote := func() {
		if !haveNote {
			return
		}
		body := trimBlankLines(noteLines)
		curNotes = append(curNotes, store.Note{
			ID:   store.NoteID(store.NewID()),
			Body: strings.Join(body, "\n"),
			Done: noteDone,
		})
		haveNote = false
		noteLines = nil
	}

	commitSection := func() {
		if curTitle == "" && len(curNotes) == 0 {
			return
		}
		sections = append(sections, store.Section{
			ID:    store.SectionID(store.NewID()),
			Title: curTitle,
			Notes: curNotes,
		})
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			pendingBlanks = append(pendingBlanks, line)
			continue
		}

		if haveNote && leadingSpaces(line) >= 2 {
			for range pendingBlanks {
				noteLines = append(noteLines, "")
			}
			pendingBlanks = nil
			noteLines = append(noteLines, line[2:])
			continue
		}
		pendingBlanks = nil

		commitNote()

		if title, ok := matchHeading(line); ok {
			commitSection()
			curTitle = title
			curNotes = nil
			continue
		}

		switch {
		case strings.HasPrefix(line, "- [ ] "):
			noteDone = false
			noteLines = []string{line[len("- [ ] "):]}
		case strings.HasPrefix(line, "- [x] "):
			noteDone = true
			noteLines = []string{line[len("- [x] "):]}
		case strings.HasPrefix(line, "- [X] "):
			noteDone = true
			noteLines = []string{line[len("- [X] "):]}
		case strings.HasPrefix(line, "- "):
			noteDone = false
			noteLines = []string{line[len("- "):]}
		case strings.HasPrefix(line, "* "):
			noteDone = false
			noteLines = []string{line[len("* "):]}
		default:
			noteDone = false
			noteLines = []string{line}
		}
		haveNote = true
	}
	commitNote()
	commitSection()

	return sections
}

// matchHeading reports whether line is a "## " section heading
// (^##[ \t]+(.*)$) and, if so, returns its title.
func matchHeading(line string) (string, bool) {
	if !strings.HasPrefix(line, "##") {
		return "", false
	}
	rest := line[2:]
	i := 0
	for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t') {
		i++
	}
	if i == 0 {
		return "", false
	}
	return rest[i:], true
}

func leadingSpaces(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

func trimBlankLines(lines []string) []string {
	start := 0
	for start < len(lines) && lines[start] == "" {
		start++
	}
	end := len(lines)
	for end > start && lines[end-1] == "" {
		end--
	}
	return lines[start:end]
}

func isValidID(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Format renders a Project into canonical bytes: front matter (only
// when the project carries an id, title, created time, or unknown
// front-matter keys), a blank line, each section (named sections get a
// "## Title" heading; the lead section — Title == "" — gets none), and
// its notes, with exactly one blank line between every block and a
// single trailing newline. It returns ErrSchemaTooNew if p.Schema is
// newer than this package understands.
func Format(p store.Project) ([]byte, error) {
	if p.Schema > CurrentSchema {
		return nil, fmt.Errorf("%w: got %d, max %d", ErrSchemaTooNew, p.Schema, CurrentSchema)
	}

	var blocks []string

	hasFrontMatter := p.ID != "" || p.Title != "" || !p.Created.IsZero() || len(p.Extra) > 0
	if hasFrontMatter {
		var b strings.Builder
		b.WriteString("---\n")
		fmt.Fprintf(&b, "ingot: %d\n", p.Schema)
		if p.ID != "" {
			fmt.Fprintf(&b, "id: %s\n", p.ID)
		}
		if p.Title != "" {
			fmt.Fprintf(&b, "title: %s\n", p.Title)
		}
		if !p.Created.IsZero() {
			fmt.Fprintf(&b, "created: %s\n", p.Created.UTC().Format(time.RFC3339))
		}
		keys := make([]string, 0, len(p.Extra))
		for k := range p.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "%s: %s\n", k, p.Extra[k])
		}
		b.WriteString("---")
		blocks = append(blocks, b.String())
	}

	for _, sec := range p.Sections {
		if sec.Title != "" {
			blocks = append(blocks, "## "+sec.Title)
		}
		for _, n := range sec.Notes {
			blocks = append(blocks, formatNote(n))
		}
	}

	if len(blocks) == 0 {
		return []byte{}, nil
	}
	return []byte(strings.Join(blocks, "\n\n") + "\n"), nil
}

func formatNote(n store.Note) string {
	marker := "- [ ] "
	if n.Done {
		marker = "- [x] "
	}
	// A bare \r would otherwise sit unescaped next to the \n Format adds
	// between blocks, indistinguishable on the next Parse from a CRLF
	// line ending — normalize it the same way Parse does, so text
	// carrying foreign line endings (a paste, a captured selection)
	// survives a save/reload cycle unchanged.
	body := strings.ReplaceAll(n.Body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	lines := strings.Split(body, "\n")

	var b strings.Builder
	b.WriteString(marker)
	b.WriteString(lines[0])
	for _, l := range lines[1:] {
		b.WriteString("\n")
		if l != "" {
			b.WriteString("  ")
			b.WriteString(l)
		}
	}
	return b.String()
}
