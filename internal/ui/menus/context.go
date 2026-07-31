package menus

import (
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

// ContextInfo is the note context menu's per-invocation state, computed
// by the caller from whichever row or selection was right-clicked,
// immediately before calling BuildContext. BuildContext only shapes
// labels and structure from it; per-item enablement (Expand, Merge Notes)
// still has to go through the corresponding Actions field, since a GMenu
// item's sensitivity always follows its bound action, not the model.
type ContextInfo struct {
	Done     bool
	Expanded bool

	CurrentProjectID string
	CurrentSectionID string
	// Sections are the current project's sections, in display order.
	Sections []Section
	// OtherProjects are every project except CurrentProjectID, in
	// display order.
	OtherProjects []Project
}

// BuildContext builds the note context menu: group 1 (Copy, Copy as
// List), group 2 (Mark as Done/Not Done, Expand/Collapse), group 3 (Edit,
// Edit in New Window, Merge Notes, Move to), in that exact order and
// grouping, each group separated by an unlabelled AppendSection.
func BuildContext(info ContextInfo) *gio.Menu {
	menu := gio.NewMenu()

	group1 := gio.NewMenu()
	group1.Append("Copy", "app.copy")
	group1.Append("Copy as List", "app.copy-as-list")
	menu.AppendSection("", group1)

	group2 := gio.NewMenu()
	group2.Append(markDoneLabel(info.Done), "app.mark-done")
	group2.Append(expandLabel(info.Expanded), "app.expand")
	menu.AppendSection("", group2)

	group3 := gio.NewMenu()
	group3.Append("Edit", "app.edit")
	group3.Append("Edit in New Window", "app.edit-new-window")
	group3.Append("Merge Notes", "app.merge")
	group3.AppendSubmenu("Move to", BuildMoveToSubmenu(info))
	menu.AppendSection("", group3)

	return menu
}

func markDoneLabel(done bool) string {
	if done {
		return "Mark as Not Done"
	}
	return "Mark as Done"
}

func expandLabel(expanded bool) string {
	if expanded {
		return "Collapse"
	}
	return "Expand"
}

// BuildMoveToSubmenu builds the Move to submenu, built fresh at every
// invocation from live project/section data: sections of the current
// project first (the current section gets a checkmark-prefixed label and
// is bound to moveToCurrentSectionAction, a name deliberately never
// registered — GTK renders an item insensitive when its action name does
// not exist, unlike an item with no action at all, which renders
// sensitive but inert; see the package doc), then a separator, then every
// other project (moving there drops the note into that project's first
// section), then a separator, then "New Section...", whose item carries
// no action but the "custom" attribute newSectionCustomID for the caller
// to fill with a real entry widget via (*gtk.PopoverMenu).AddChild after
// every rebuild — see ContextMenuController.SetOnRebuilt.
func BuildMoveToSubmenu(info ContextInfo) *gio.Menu {
	sections := gio.NewMenu()
	for _, s := range info.Sections {
		if s.ID == info.CurrentSectionID {
			sections.Append("✓ "+s.Title, moveToCurrentSectionAction)
			continue
		}
		item := gio.NewMenuItem(s.Title, "")
		item.SetActionAndTargetValue("app.move-to", glib.NewVariantString("section:"+s.ID))
		sections.AppendItem(item)
	}

	projects := gio.NewMenu()
	for _, p := range info.OtherProjects {
		item := gio.NewMenuItem(p.Title, "")
		item.SetActionAndTargetValue("app.move-to", glib.NewVariantString("project:"+p.ID))
		projects.AppendItem(item)
	}

	newSection := gio.NewMenu()
	newSectionItem := gio.NewMenuItem("New Section...", "")
	newSectionItem.SetAttributeValue("custom", glib.NewVariantString(newSectionCustomID))
	newSection.AppendItem(newSectionItem)

	menu := gio.NewMenu()
	menu.AppendSection("", sections)
	menu.AppendSection("", projects)
	menu.AppendSection("", newSection)
	return menu
}
