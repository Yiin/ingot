package menus

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

func baseContextInfo() ContextInfo {
	return ContextInfo{
		CurrentProjectID: "p1",
		CurrentSectionID: "s1",
		Sections: []Section{
			{ID: "s1", Title: "Today"},
			{ID: "s2", Title: "Later"},
		},
		OtherProjects: []Project{
			{ID: "p2", Title: "Work"},
		},
	}
}

func itemLabel(t *testing.T, m interface {
	ItemAttributeValue(int, string, *glib.VariantType) *glib.Variant
}, idx int) string {
	t.Helper()
	v := m.ItemAttributeValue(idx, "label", glib.NewVariantType("s"))
	if v == nil {
		t.Fatalf("item %d has no label attribute", idx)
	}
	return v.String()
}

// TestBuildContextGroupsAndItemCounts checks the note context menu has
// exactly the three sections, in the spec's exact grouping and order:
// group 1 (Copy, Copy as List), group 2 (Mark as Done, Expand), group 3
// (Edit, Edit in New Window, Merge Notes, Move to).
func TestBuildContextGroupsAndItemCounts(t *testing.T) {
	menu := BuildContext(baseContextInfo())

	if n := menu.NItems(); n != 3 {
		t.Fatalf("top-level menu has %d items, want 3 sections", n)
	}

	wantCounts := []int{2, 2, 4}
	wantLabels := [][]string{
		{"Copy", "Copy as List"},
		{"Mark as Done", "Expand"},
		{"Edit", "Edit in New Window", "Merge Notes", "Move to"},
	}
	for i, wantCount := range wantCounts {
		link := menu.ItemLink(i, "section")
		if link == nil {
			t.Fatalf("item %d has no section link", i)
		}
		model := gio.BaseMenuModel(link)
		if n := model.NItems(); n != wantCount {
			t.Errorf("section %d has %d items, want %d", i, n, wantCount)
		}
		for j, wantLabel := range wantLabels[i] {
			if got := itemLabel(t, model, j); got != wantLabel {
				t.Errorf("section %d item %d label = %q, want %q", i, j, got, wantLabel)
			}
		}
	}
}

func TestBuildContextMarkDoneLabelFlips(t *testing.T) {
	info := baseContextInfo()
	info.Done = true
	menu := BuildContext(info)

	group2 := gio.BaseMenuModel(menu.ItemLink(1, "section"))
	if got := itemLabel(t, group2, 0); got != "Mark as Not Done" {
		t.Errorf("done row's mark-done label = %q, want %q", got, "Mark as Not Done")
	}
}

func TestBuildContextExpandLabelFlips(t *testing.T) {
	info := baseContextInfo()
	info.Expanded = true
	menu := BuildContext(info)

	group2 := gio.BaseMenuModel(menu.ItemLink(1, "section"))
	if got := itemLabel(t, group2, 1); got != "Collapse" {
		t.Errorf("expanded row's expand label = %q, want %q", got, "Collapse")
	}
}

// TestBuildMoveToSubmenuStructure checks the Move to submenu produces one
// item per section (current section ticked, unbound and so insensitive;
// other sections bound to app.move-to), one item per other project, and
// the New Section... custom item, each in its own separator-delimited
// group.
func TestBuildMoveToSubmenuStructure(t *testing.T) {
	info := baseContextInfo()
	menu := BuildMoveToSubmenu(info)

	if n := menu.NItems(); n != 3 {
		t.Fatalf("Move to submenu has %d top-level items, want 3 sections", n)
	}

	sections := gio.BaseMenuModel(menu.ItemLink(0, "section"))
	if n := sections.NItems(); n != len(info.Sections) {
		t.Fatalf("sections group has %d items, want %d (one per section)", n, len(info.Sections))
	}
	if got := itemLabel(t, sections, 0); got != "✓ Today" {
		t.Errorf("current section label = %q, want checkmark-prefixed", got)
	}
	// Bound to a deliberately unregistered action name, not left
	// unbound — see doc.go and moveToCurrentSectionAction's own comment
	// for why an unbound item would render sensitive, not insensitive.
	action := sections.ItemAttributeValue(0, "action", glib.NewVariantType("s"))
	if action == nil || action.String() != moveToCurrentSectionAction {
		t.Errorf("current section item action = %v, want %q", action, moveToCurrentSectionAction)
	}
	target := sections.ItemAttributeValue(1, "target", glib.NewVariantType("s"))
	if target == nil || target.String() != "section:s2" {
		t.Errorf("other section target = %v, want %q", target, "section:s2")
	}

	projects := gio.BaseMenuModel(menu.ItemLink(1, "section"))
	if n := projects.NItems(); n != len(info.OtherProjects) {
		t.Fatalf("projects group has %d items, want %d", n, len(info.OtherProjects))
	}
	projTarget := projects.ItemAttributeValue(0, "target", glib.NewVariantType("s"))
	if projTarget == nil || projTarget.String() != "project:p2" {
		t.Errorf("project target = %v, want %q", projTarget, "project:p2")
	}

	newSection := gio.BaseMenuModel(menu.ItemLink(2, "section"))
	if n := newSection.NItems(); n != 1 {
		t.Fatalf("new-section group has %d items, want 1", n)
	}
	custom := newSection.ItemAttributeValue(0, "custom", glib.NewVariantType("s"))
	if custom == nil || custom.String() != NewSectionCustomID {
		t.Errorf("New Section item custom attribute = %v, want %q", custom, NewSectionCustomID)
	}
}
