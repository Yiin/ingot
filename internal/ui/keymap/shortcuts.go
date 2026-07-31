package keymap

import (
	"encoding/xml"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// windowID is the GtkBuilder object id UI() gives the top-level
// GtkShortcutsWindow, and NewShortcutsWindow looks back up.
const windowID = "ingot-shortcuts-window"

// GroupSection is one GtkShortcutsSection's worth of entries, grouped
// by Entry.Group in Groups' fixed display order — the pure
// representation BuildSections produces, testable without a live GTK
// display. UI() turns the same data into the real widget's GtkBuilder
// XML.
type GroupSection struct {
	Group Group
	Rows  []Entry
}

// BuildSections groups entries by Group, in Groups' fixed display
// order. An entry whose Group is not one of Groups is silently
// dropped, which is exactly the mistake TestBuildSectionsCoversEveryEntry
// exists to catch.
func BuildSections(entries []Entry) []GroupSection {
	byGroup := make(map[Group][]Entry, len(Groups))
	for _, e := range entries {
		byGroup[e.Group] = append(byGroup[e.Group], e)
	}
	sections := make([]GroupSection, 0, len(Groups))
	for _, g := range Groups {
		if rows, ok := byGroup[g]; ok {
			sections = append(sections, GroupSection{Group: g, Rows: rows})
		}
	}
	return sections
}

// UI returns the GtkBuilder XML NewShortcutsWindow parses to build the
// real GtkShortcutsWindow, generated from Table via BuildSections. It
// is exported so a test can check the generated markup is well-formed
// and covers every Table entry without needing a live GTK display —
// see shortcuts_test.go.
func UI() string {
	sections := BuildSections(Table)

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<interface>` + "\n")
	b.WriteString(`  <object class="GtkShortcutsWindow" id="` + windowID + `">` + "\n")
	b.WriteString(`    <property name="modal">1</property>` + "\n")
	for _, sec := range sections {
		b.WriteString(`    <child>` + "\n")
		b.WriteString(`      <object class="GtkShortcutsSection">` + "\n")
		b.WriteString(`        <property name="section-name">` + escapeXML(string(sec.Group)) + `</property>` + "\n")
		b.WriteString(`        <child>` + "\n")
		b.WriteString(`          <object class="GtkShortcutsGroup">` + "\n")
		b.WriteString(`            <property name="title">` + escapeXML(string(sec.Group)) + `</property>` + "\n")
		for _, e := range sec.Rows {
			b.WriteString(`            <child>` + "\n")
			b.WriteString(`              <object class="GtkShortcutsShortcut">` + "\n")
			b.WriteString(`                <property name="title">` + escapeXML(e.Title) + `</property>` + "\n")
			if len(e.Accels) > 0 {
				b.WriteString(`                <property name="accelerator">` + escapeXML(strings.Join(e.Accels, " ")) + `</property>` + "\n")
			} else if e.Display != "" {
				b.WriteString(`                <property name="subtitle">` + escapeXML(e.Display) + `</property>` + "\n")
			}
			b.WriteString(`              </object>` + "\n")
			b.WriteString(`            </child>` + "\n")
		}
		b.WriteString(`          </object>` + "\n")
		b.WriteString(`        </child>` + "\n")
		b.WriteString(`      </object>` + "\n")
		b.WriteString(`    </child>` + "\n")
	}
	b.WriteString(`  </object>` + "\n")
	b.WriteString(`</interface>` + "\n")
	return b.String()
}

func escapeXML(s string) string {
	var b strings.Builder
	// xml.EscapeText never errors writing to a strings.Builder.
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// NewShortcutsWindow builds the real GtkShortcutsWindow from Table, via
// UI(). Like the rest of internal/ui's GTK-attachment code, it is
// exercised by the running app, not by go test, which has no display to
// build real widgets against — see the package doc.
func NewShortcutsWindow() *gtk.ShortcutsWindow {
	builder := gtk.NewBuilderFromString(UI())
	return builder.GetObject(windowID).Cast().(*gtk.ShortcutsWindow)
}
