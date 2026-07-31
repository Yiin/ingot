package mdpango

import (
	"regexp"
	"strings"
	"testing"

	"github.com/diamondburned/gotk4/pkg/pango"
)

func TestGolden(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"bold", "**bold**", "<b>bold</b>"},
		{"code", "`code`", "<tt>code</tt>"},
		{"italic", "*italic*", "<i>italic</i>"},
		{
			"link with query params",
			"[text](https://example.com/path?a=1&b=2)",
			`<a href="https://example.com/path?a=1&amp;b=2">text</a>`,
		},
		{"star multiplication is not emphasis", "5 * 3 * 2 = 30", "5 * 3 * 2 = 30"},
		{"underscores inside a word are not emphasis", "snake_case_name_here", "snake_case_name_here"},
		{"an img tag is escaped, not interpreted", "<img src=x onerror=alert(1)>", "&lt;img src=x onerror=alert(1)&gt;"},
		{"a NUL byte is stripped, not truncating", "nul\x00after", "nulafter"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Full(c.body); got != c.want {
				t.Errorf("Full(%q) = %q, want %q", c.body, got, c.want)
			}
			if got := Safe(c.body); got != c.want {
				t.Errorf("Safe(%q) = %q, want %q", c.body, got, c.want)
			}
		})
	}
}

func TestCollapsedNeverContainsNewline(t *testing.T) {
	cases := []string{
		"para one\n\npara two\n\npara three",
		"- one\n- two\n- three",
		"```\nline one\nline two\n```",
		"<div>\n<p>raw</p>\n</div>",
		"line one\nline two soft break",
	}
	for _, body := range cases {
		if got := Collapsed(body); strings.Contains(got, "\n") {
			t.Errorf("Collapsed(%q) = %q, contains a newline", body, got)
		}
	}
}

// hostileBodies are bodies chosen to break a naive renderer: HTML injection
// attempts, delimiter ambiguity, control bytes, and malformed markdown.
var hostileBodies = []string{
	"**bold**",
	"`code`",
	"*italic*",
	"[text](https://example.com/path?a=1&b=2)",
	"5 * 3 * 2 = 30",
	"snake_case_name_here",
	"<img src=x onerror=alert(1)>",
	"nul\x00after",
	`[x](" onclick="evil)`,
	"<script>alert(1)</script>",
	`<a href="javascript:alert(1)">click</a>`,
	"***bold italic***",
	"a\x01\x02\x03b",
	"*unterminated",
	"**[bold link](http://example.com)**",
	"<div>\n<p>raw</p>\n</div>\n\nafter",
	"<http://example.com/path?x=1&y=2>",
	"# Heading text",
	"- one\n- two\n- three",
	"```\n<script>bad</script>\n```",
	"AT&T",
	"a < b and a > b",
	`![alt" onerror="x](http://evil.com)`,
	"&amp;&lt;&gt;",
	"café 🎉 <b>not-a-tag</b>",
	strings.Repeat("*", 200) + "text" + strings.Repeat("*", 200),
	`[click](https://x.com/"><script>alert(1)</script>)`,
}

// allowedTag matches the only tags mdpango ever emits itself. Whatever is
// left after stripping them must contain no '<' — every '<' that came from
// the input body has to have been escaped to &lt;.
var allowedTag = regexp.MustCompile(`</?(b|i|tt|s|u)>|<span[^>]*>|</span>|<a href="[^"]*">|</a>`)

func TestSafeSurvivesHostileInput(t *testing.T) {
	if len(hostileBodies) < 17 {
		t.Fatalf("only %d hostile bodies, want at least 17", len(hostileBodies))
	}
	for _, body := range hostileBodies {
		t.Run(body, func(t *testing.T) {
			markup := Safe(body)

			stripped := anchorTag.ReplaceAllString(markup, "")
			if _, _, _, err := pango.ParseMarkup(stripped, 0); err != nil {
				t.Fatalf("Safe(%q) = %q, pango.ParseMarkup after anchor-stripping: %v", body, markup, err)
			}

			remainder := allowedTag.ReplaceAllString(markup, "")
			if strings.Contains(remainder, "<") {
				t.Fatalf("Safe(%q) = %q, an unescaped < from the input survived", body, markup)
			}
		})
	}
}

func TestSafeCollapsedNeverContainsNewline(t *testing.T) {
	for _, body := range hostileBodies {
		markup := SafeCollapsed(body)
		if strings.Contains(markup, "\n") {
			t.Errorf("SafeCollapsed(%q) = %q, contains a newline", body, markup)
		}
		stripped := anchorTag.ReplaceAllString(markup, "")
		if _, _, _, err := pango.ParseMarkup(stripped, 0); err != nil {
			t.Fatalf("SafeCollapsed(%q) = %q, pango.ParseMarkup after anchor-stripping: %v", body, markup, err)
		}
	}
}

func FuzzSafe(f *testing.F) {
	for _, body := range hostileBodies {
		f.Add(body)
	}
	f.Fuzz(func(t *testing.T, body string) {
		markup := Safe(body)
		stripped := anchorTag.ReplaceAllString(markup, "")
		if _, _, _, err := pango.ParseMarkup(stripped, 0); err != nil {
			t.Fatalf("Safe(%q) = %q, pango.ParseMarkup after anchor-stripping: %v", body, markup, err)
		}
	})
}
