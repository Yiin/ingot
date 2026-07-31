package menus

import (
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

// BuildOverflow builds the overflow menu: the project list (a tick on the
// active project comes free from "app.project"'s own state — see the
// package doc — and Ctrl+1..9 accelerators come from
// (*Actions).SetProjectAccels, called separately whenever the project
// list changes), a separator, Clear Done (a checkmark from
// "app.clear-done"'s own boolean state communicates its armed/confirming
// state — see Actions.ClearDoneArmed — so this menu needs no rebuild
// just to reflect it), a separator, Keyboard Shortcuts, a separator,
// then the "Window" group — rendered as a genuine GMenu section with
// label "Window" rather than an item, since a real section heading is
// the correct GTK idiom for a non-interactive label (an item with no
// bound action renders sensitive, not disabled) — holding Keep on Top
// (a checkmark from "app.keep-on-top"'s own state) and Close.
func BuildOverflow(projects []Project) *gio.Menu {
	projectsSection := gio.NewMenu()
	for _, p := range projects {
		item := gio.NewMenuItem(p.Title, "")
		item.SetActionAndTargetValue("app.project", glib.NewVariantString(p.ID))
		projectsSection.AppendItem(item)
	}

	clearDone := gio.NewMenu()
	clearDone.Append("Clear Done", "app.clear-done")

	shortcuts := gio.NewMenu()
	shortcuts.Append("Keyboard Shortcuts", "app.shortcuts")

	window := gio.NewMenu()
	window.Append("Keep on Top", "app.keep-on-top")
	window.Append("Close", "app.close")

	menu := gio.NewMenu()
	menu.AppendSection("", projectsSection)
	menu.AppendSection("", clearDone)
	menu.AppendSection("", shortcuts)
	menu.AppendSection("Window", window)
	return menu
}
