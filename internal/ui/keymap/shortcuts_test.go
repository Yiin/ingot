package keymap

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestBuildSectionsCoversEveryEntry(t *testing.T) {
	sections := BuildSections(Table)

	got := 0
	for _, s := range sections {
		got += len(s.Rows)
	}
	if got != len(Table) {
		t.Fatalf("BuildSections(Table) covers %d entries, want %d (one per Table entry)", got, len(Table))
	}

	// Groups' fixed order must be respected: each section's Group must
	// appear no earlier than the previous section's, and in exactly the
	// order Groups declares.
	rank := make(map[Group]int, len(Groups))
	for i, g := range Groups {
		rank[g] = i
	}
	last := -1
	for _, s := range sections {
		r := rank[s.Group]
		if r <= last {
			t.Errorf("section %q is out of Groups order (rank %d after rank %d)", s.Group, r, last)
		}
		last = r
	}
}

// xmlObject mirrors just enough of the GtkBuilder <object> schema to
// walk UI()'s generated tree without a live GTK display.
type xmlObject struct {
	Class      string        `xml:"class,attr"`
	Properties []xmlProperty `xml:"property"`
	Children   []struct {
		Object xmlObject `xml:"object"`
	} `xml:"child"`
}

type xmlProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

func (o xmlObject) property(name string) (string, bool) {
	for _, p := range o.Properties {
		if p.Name == name {
			return strings.TrimSpace(p.Value), true
		}
	}
	return "", false
}

// collectShortcuts walks o's tree and returns every GtkShortcutsShortcut
// object's title.
func collectShortcuts(o xmlObject) map[string]bool {
	out := make(map[string]bool)
	if o.Class == "GtkShortcutsShortcut" {
		if title, ok := o.property("title"); ok {
			out[title] = true
		}
	}
	for _, c := range o.Children {
		for title := range collectShortcuts(c.Object) {
			out[title] = true
		}
	}
	return out
}

// TestUIIsWellFormedAndMatchesTableBidirectionally is the acceptance
// criteria's "every keymap entry appears in the shortcuts window and
// every installed GtkShortcut has a matching table entry, bidirectionally,
// with no orphans" — checked as a pure structural round-trip, since
// go test has no live display to build the real GtkShortcutsWindow
// against (see the package doc and NewShortcutsWindow).
func TestUIIsWellFormedAndMatchesTableBidirectionally(t *testing.T) {
	var root struct {
		Object xmlObject `xml:"object"`
	}
	raw := UI()
	if err := xml.Unmarshal([]byte(raw), &root); err != nil {
		t.Fatalf("UI() is not well-formed XML: %v\n%s", err, raw)
	}
	if root.Object.Class != "GtkShortcutsWindow" {
		t.Fatalf("UI()'s top-level object is %q, want GtkShortcutsWindow", root.Object.Class)
	}

	got := collectShortcuts(root.Object)

	want := make(map[string]bool, len(Table))
	for _, e := range Table {
		want[e.Title] = true
	}

	if len(got) != len(Table) {
		t.Errorf("UI() rendered %d GtkShortcutsShortcut rows, want %d (one per Table entry)", len(got), len(Table))
	}
	for title := range got {
		if !want[title] {
			t.Errorf("UI() has a row %q with no matching Table entry (orphan)", title)
		}
	}
	for title := range want {
		if !got[title] {
			t.Errorf("Table entry %q has no matching row in UI()", title)
		}
	}
}
