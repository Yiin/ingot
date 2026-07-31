package app

import (
	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/ui/keymap"
	"github.com/Yiin/ingot/internal/ui/notelist"
)

// wireNav builds keymap.Nav over the panel's current display order and
// installs it on the list's real GtkListView (see keymap.InstallNav's
// doc comment for exactly which keys this takes over from GTK's own
// default handling), keeping Nav's own row order in sync with every
// store-driven model change and its focus in sync with a selection
// change Nav didn't itself drive (a mouse click — see Nav.SyncFocus).
func (a *App) wireNav() {
	list := a.shell.List()
	a.nav = keymap.NewNav(a.navRows())

	keymap.InstallNav(list.ListView(), a.nav, safe("nav-sync", a.syncNavToList))
	list.ConnectSelectionChanged(safe("list-selection-changed", a.onListSelectionChanged))

	a.adapter.onRowsChanged = a.syncNavRows
	a.shell.OnFilterChanged(a.syncNavRows)
}

// navRows reads the list's current sorted (displayed) order as
// keymap.Row, skipping the placeholder every empty section carries —
// Nav has no notion of a placeholder row.
func (a *App) navRows() []keymap.Row {
	list := a.shell.List()
	n := list.ViewLen()
	rows := make([]keymap.Row, 0, n)
	for i := 0; i < n; i++ {
		it := list.ItemAtViewPosition(i)
		if it == nil {
			continue
		}
		rows = append(rows, keymap.Row{ID: it.ID, SectionID: it.SectionID})
	}
	return rows
}

// syncNavRows re-reads the display order into Nav, preserving focus,
// the anchor, and the selection by row ID wherever that ID still exists
// (see Nav.SetRows). Wired as the adapter's onRowsChanged hook, so it
// runs after every store event that can change row membership or order.
func (a *App) syncNavRows() {
	if a.nav == nil {
		return
	}
	a.nav.SetRows(a.navRows())
}

// syncNavToList is keymap.InstallNav's onChanged callback: it pushes
// Nav's own focus/selection state back onto the real GtkSelectionModel,
// anchor ring, and scroll position after a Nav-driven key press. Guarded
// by syncingNavToList (reset via defer, so a panic mid-sync can't leave
// it stuck true) so the SelectItems call below's own
// ConnectSelectionChanged echo does not immediately try to re-derive
// Nav's own selection from the selection it was itself just given — see
// onListSelectionChanged.
func (a *App) syncNavToList() {
	list := a.shell.List()

	ids := a.nav.Selected()
	items := make([]*notelist.Item, 0, len(ids))
	for _, id := range ids {
		if it := a.adapter.itemForNote(store.NoteID(id)); it != nil {
			items = append(items, it)
		}
	}

	a.syncingNavToList = true
	func() {
		defer func() { a.syncingNavToList = false }()
		list.SelectItems(items)
	}()

	if focusedID := a.nav.FocusedID(); focusedID != "" {
		if it := a.adapter.itemForNote(store.NoteID(focusedID)); it != nil {
			list.SetAnchor(it)
			list.ScrollTo(it)
			return
		}
	}
	list.SetAnchor(nil)
}

// onListSelectionChanged is the real GtkSelectionModel's own
// selection-changed signal, which fires both for Nav-driven changes
// (syncNavToList's own SelectItems call, ignored here — see
// syncingNavToList) and for anything else that changes it — chiefly the
// list's own default mouse click handling, but also
// ContextMenuController's right-click targeting. RefreshMergeEnablement
// must run on every such change regardless of cause (see its own doc
// comment); Nav.SyncSelection only for the latter, so a keyboard move
// that follows continues from what's actually selected on screen
// instead of whatever Nav's own selection map last computed — without
// this, Nav's map only ever reflects Nav's own prior moves, so e.g. a
// mouse click followed immediately by Ctrl+A would select-all against a
// stale (possibly empty) base set instead of the row just clicked.
// SyncSelection itself syncs focus to ids' last entry (list.Selected()'s
// own view order — a close but not exact proxy for "the row the mouse
// most recently touched": exact for a plain click, and for a Shift/Ctrl-
// click that happens to land at the bottom of the resulting selection,
// but not guaranteed otherwise).
func (a *App) onListSelectionChanged() {
	if a.menuActions != nil {
		a.menuActions.RefreshMergeEnablement(len(a.shell.List().Selected()))
	}
	if a.syncingNavToList || a.nav == nil {
		return
	}
	selected := a.shell.List().Selected()
	ids := make([]string, len(selected))
	for i, it := range selected {
		ids[i] = it.ID
	}
	a.nav.SyncSelection(ids)
}
