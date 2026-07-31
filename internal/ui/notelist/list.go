package notelist

import (
	"time"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/motion"
	"github.com/Yiin/ingot/internal/ui/theme"
	"github.com/Yiin/ingot/internal/ui/widget"
)

// rowBinding is the state kept alongside one recycled ListItem's
// widget.Row for as long as the factory keeps that ListItem alive —
// built once in Setup, refreshed on every Bind.
type rowBinding struct {
	li  *gtk.ListItem
	row *widget.Row
	ph  *gtk.Label // "No notes yet" placeholder card; exactly one of row/ph is visible

	item     *Item
	suppress bool // true while Bind is driving Row's setters programmatically
	strip    glib.SourceHandle
	flash    glib.SourceHandle
}

func (b *rowBinding) cancelStrip() {
	if b.strip != 0 {
		glib.SourceRemove(b.strip)
		b.strip = 0
	}
}

func (b *rowBinding) cancelFlash() {
	if b.flash != 0 {
		glib.SourceRemove(b.flash)
		b.flash = 0
	}
}

// headerBinding is the state kept alongside one recycled ListHeader's
// sectionHeader widget.
type headerBinding struct {
	header *sectionHeader
}

// List is the panel's scrolling body: section headers over note cards,
// multi-selectable, with an overlay scrollbar and an insert animation for
// freshly captured notes. Callers mutate its contents through Model();
// List itself only ever reads it to drive GTK's object graph.
type List struct {
	*gtk.Overlay

	model *Model

	filter        *gtk.CustomFilter
	filterModel   *gtk.FilterListModel
	filterPred    func(it *Item) bool
	orderSorter   *gtk.CustomSorter
	sectionSorter *gtk.CustomSorter
	sort          *gtk.SortListModel
	sel           *gtk.MultiSelection
	listView      *gtk.ListView
	scrolled      *gtk.ScrolledWindow
	scrollbar     *overlayScrollbar

	rowFactory *gtk.SignalListItemFactory
	hdrFactory *gtk.SignalListItemFactory

	rows    map[uintptr]*rowBinding
	headers map[uintptr]*headerBinding

	anchor *Item

	onToggle           func(it *Item, done bool)
	onSelectionChanged func()
	onActivate         func(it *Item)

	// Test hooks. Only ever set from list_integration_test.go, which
	// needs a live GTK display and does not run in this sandbox — see
	// doc.go.
	onItemSetup  func()
	onHeaderBind func(start, n uint)
}

// New builds a List over sections, in the given display order, each
// starting empty (and so each carrying its own placeholder card).
func New(sections []Section) *List {
	m := NewModel(sections)

	l := &List{
		model:   m,
		rows:    make(map[uintptr]*rowBinding),
		headers: make(map[uintptr]*headerBinding),
	}

	// model -> filter model (internal/ui/search's live query, copper-l2z.28)
	// -> sort model (section sorter + item sorter, same comparator family,
	// see order.go) -> multi-selection -> list view.
	l.filter = gtk.NewCustomFilter(l.filterFunc)
	l.filterModel = gtk.NewFilterListModel(m.ListModel(), &l.filter.Filter)

	l.orderSorter = gtk.NewCustomSorter(sorterFunc(m.compareOrder))
	l.sectionSorter = gtk.NewCustomSorter(sorterFunc(m.compareSection))
	m.invalidate = l.invalidateSort

	l.sort = gtk.NewSortListModel(l.filterModel, &l.orderSorter.Sorter)
	l.sort.SetSectionSorter(&l.sectionSorter.Sorter)

	l.sel = gtk.NewMultiSelection(l.sort)
	l.sel.ConnectSelectionChanged(func(pos, n uint) {
		l.repaintSelection()
		if l.onSelectionChanged != nil {
			l.onSelectionChanged()
		}
	})

	l.rowFactory = gtk.NewSignalListItemFactory()
	l.rowFactory.ConnectSetup(l.setupRow)
	l.rowFactory.ConnectBind(l.bindRow)
	l.rowFactory.ConnectUnbind(l.unbindRow)
	l.rowFactory.ConnectTeardown(l.teardownRow)

	l.hdrFactory = gtk.NewSignalListItemFactory()
	l.hdrFactory.ConnectSetup(l.setupHeader)
	l.hdrFactory.ConnectBind(l.bindHeader)
	l.hdrFactory.ConnectTeardown(l.teardownHeader)

	lv := gtk.NewListView(l.sel, &l.rowFactory.ListItemFactory)
	lv.SetHeaderFactory(&l.hdrFactory.ListItemFactory)
	lv.SetTabBehavior(gtk.ListTabItem)
	lv.AddCSSClass("ingot-notelist")
	lv.SetShowSeparators(false)
	lv.SetSingleClickActivate(false)
	lv.SetVExpand(true)
	lv.ConnectActivate(func(pos uint) {
		if l.onActivate == nil {
			return
		}
		it := itemModel.ObjectValue(l.sel.Item(pos))
		if it.IsPlaceholder() {
			return
		}
		l.onActivate(it)
	})
	l.listView = lv

	sw := gtk.NewScrolledWindow()
	sw.SetChild(lv)
	// PolicyExternal: GTK still lets the content scroll but draws no
	// scrollbar of its own — this package draws its own overlay bar
	// instead, so it can control the fade timing and 5dp geometry the
	// child spec asks for.
	sw.SetPolicy(gtk.PolicyNever, gtk.PolicyExternal)
	sw.SetOverlayScrolling(false)
	sw.SetVExpand(true)
	sw.SetHExpand(true)
	sw.SetPropagateNaturalHeight(false)
	l.scrolled = sw

	sb := newOverlayScrollbar(sw.VAdjustment())
	sw.VAdjustment().ConnectValueChanged(func() { sb.poke(sw.VAdjustment()) })
	l.scrollbar = sb

	// The scrollbar is an overlay child of the ScrolledWindow, not of
	// the whole panel — its allocation is therefore bounded by the
	// scrolled window's own, which stops above wherever the panel packs
	// the composer next, never inside it.
	root := gtk.NewOverlay()
	root.SetChild(sw)
	root.AddOverlay(sb.Scrollbar)
	root.SetMeasureOverlay(sb.Scrollbar, false)
	root.SetClipOverlay(sb.Scrollbar, false)
	l.Overlay = root

	return l
}

// Model returns the List's data model. Callers mutate notes through it
// (Append, InsertAt, RemoveAt, Move, Reset); List repaints in response.
func (l *List) Model() *Model { return l.model }

// ListView returns the underlying GtkListView, for a sibling child (e.g.
// copper-l2z.25's keymap) to attach event controllers to.
func (l *List) ListView() *gtk.ListView { return l.listView }

// Selection returns the underlying GtkMultiSelection.
func (l *List) Selection() *gtk.MultiSelection { return l.sel }

// Selected returns every currently selected real note, in view order.
// Placeholders are never selectable through this package's own API, but
// are filtered out here defensively.
func (l *List) Selected() []*Item {
	bitset := l.sel.Selection()
	var out []*Item
	iter, pos, ok := gtk.BitsetIterInitFirst(bitset)
	for ok {
		it := itemModel.ObjectValue(l.sel.Item(pos))
		if !it.IsPlaceholder() {
			out = append(out, it)
		}
		pos, ok = iter.Next()
	}
	return out
}

// SelectItems replaces the current selection with items.
func (l *List) SelectItems(items []*Item) {
	l.sel.UnselectAll()
	for _, it := range items {
		if pos := l.model.ViewPosition(it); pos >= 0 {
			l.sel.SelectItem(uint(pos), false)
		}
	}
}

// SetAnchor marks it as the keyboard-focus anchor within a
// multi-selection (widget.Row's .selection-anchor ring), repainting
// every live row.
func (l *List) SetAnchor(it *Item) {
	l.anchor = it
	l.repaintSelection()
}

// Anchor returns the current keyboard-focus anchor within the
// multi-selection, or nil if none is set.
func (l *List) Anchor() *Item { return l.anchor }

// ScrollTo scrolls it into view without changing the selection.
func (l *List) ScrollTo(it *Item) {
	if pos := l.model.ViewPosition(it); pos >= 0 {
		l.listView.ScrollTo(uint(pos), gtk.ListScrollNone, nil)
	}
}

// ScrollToAndSelect scrolls to and selects it — the typical response to a
// fresh capture landing at the top of its section.
func (l *List) ScrollToAndSelect(it *Item) {
	if pos := l.model.ViewPosition(it); pos >= 0 {
		l.listView.ScrollTo(uint(pos), gtk.ListScrollSelect|gtk.ListScrollFocus, nil)
	}
}

// ConnectSelectionChanged registers f to run whenever the live selection
// changes.
func (l *List) ConnectSelectionChanged(f func()) { l.onSelectionChanged = f }

// ConnectToggled registers f to run whenever a row's checkbox is
// clicked, real user input only — never for List's own programmatic
// rebinds.
func (l *List) ConnectToggled(f func(it *Item, done bool)) { l.onToggle = f }

// ConnectActivate registers f to run when a row is activated (Enter /
// double-click), real notes only.
func (l *List) ConnectActivate(f func(it *Item)) { l.onActivate = f }

// SetFilter installs pred as the list's live visibility predicate and
// re-runs it over every item — internal/ui/search's per-keystroke query
// recompute, or nil to show everything again (an empty query). pred is
// never asked about a placeholder card itself: the caller decides a
// section's visibility (and so its placeholder's) by SectionID instead,
// since a placeholder carries no note body to match against.
func (l *List) SetFilter(pred func(it *Item) bool) {
	l.filterPred = pred
	l.filter.Changed(gtk.FilterChangeDifferent)
}

func (l *List) filterFunc(obj *coreglib.Object) bool {
	if l.filterPred == nil {
		return true
	}
	return l.filterPred(itemModel.ObjectValue(obj))
}

// RefreshHighlights pushes every bound row's current Item.Ranges onto its
// Label — the search package's response to a query change for a note
// that stays visible across it (a plain Splice/filter re-run only
// notifies GTK about items entering or leaving the filtered set, not
// about a still-visible item's Ranges changing in place). Mirrors
// repaintSelection's same iterate-the-recycled-bindings approach.
func (l *List) RefreshHighlights() {
	for _, b := range l.rows {
		if b.item == nil || b.item.IsPlaceholder() {
			continue
		}
		b.row.Label.SetHighlight(b.item.Ranges)
	}
}

func (l *List) invalidateSort() {
	l.orderSorter.Changed(gtk.SorterChangeDifferent)
	l.sectionSorter.Changed(gtk.SorterChangeDifferent)
}

// repaintSelection pushes the live GtkSelectionModel state onto every
// bound row. Iterating the (recycled, ~205-wide even at 5000 items)
// binding map is cheaper than teaching widget.Row to watch the selection
// model itself.
func (l *List) repaintSelection() {
	for _, b := range l.rows {
		if b.item == nil || b.item.IsPlaceholder() {
			continue
		}
		b.row.SetSelected(b.li.Selected())
		b.row.SetSelectionAnchor(b.item == l.anchor)
	}
}

func (l *List) setupRow(obj *coreglib.Object) {
	li := obj.Cast().(*gtk.ListItem)

	row := widget.NewRow()

	ph := gtk.NewLabel("No notes yet")
	ph.AddCSSClass("note-placeholder")
	ph.SetHAlign(gtk.AlignFill)
	ph.SetVAlign(gtk.AlignCenter)

	wrap := gtk.NewBox(gtk.OrientationVertical, 0)
	wrap.Append(row)
	wrap.Append(ph)
	li.SetChild(wrap)

	b := &rowBinding{li: li, row: row, ph: ph}
	// Connected once here, in Setup: connecting in Bind would stack one
	// handler per recycle.
	row.Checkbox.ConnectToggled(func(checked bool) {
		if b.suppress || b.item == nil || b.item.IsPlaceholder() {
			return
		}
		b.item.Done = checked
		if l.onToggle != nil {
			l.onToggle(b.item, checked)
		}
	})

	l.rows[li.Native()] = b
	if l.onItemSetup != nil {
		l.onItemSetup()
	}
}

// bindRow is the recycling tax: every mutable property widget.Row exposes
// must be reset here, in a fixed order, or a value from the row's
// previous binding leaks onto its new one.
func (l *List) bindRow(obj *coreglib.Object) {
	li := obj.Cast().(*gtk.ListItem)
	b := l.rows[li.Native()]

	it := itemModel.ObjectValue(li.Item())
	b.item = it
	b.cancelStrip()
	b.cancelFlash()

	b.suppress = true
	b.row.RemoveCSSClass("just-inserted")   // first, unconditionally
	b.row.RemoveCSSClass("duplicate-flash") // same: never replay on recycle

	placeholder := it.IsPlaceholder()
	b.row.SetVisible(!placeholder)
	b.ph.SetVisible(placeholder)
	// A placeholder is not a note: keep it out of GtkSelectionModel's
	// selection and activation entirely, not just filtered out of this
	// package's own Selected()/activate helpers — otherwise Ctrl+A or a
	// rubber-band selection can select "nothing" at the model level.
	li.SetSelectable(!placeholder)
	li.SetActivatable(!placeholder)

	if !placeholder {
		b.row.SetExpanded(false)
		b.row.SetDragging(false)
		b.row.SetSelectionAnchor(it == l.anchor)
		b.row.SetSelected(li.Selected())
		b.row.SetChecked(false, false) // reset-then-apply kills any in-flight strike
		b.row.SetChecked(it.Done, false)
		b.row.Label.SetBody(it.Body)
		b.row.Label.SetHighlight(it.Ranges)

		// motion.EnableAnimations() gates this even though the visible
		// wipe itself is CSS (style.css's ingot-row-in @keyframes,
		// already free per GTK's own gtk-enable-animations handling):
		// with animations off, the row must never even carry the class,
		// so its very first bound frame already shows the resting
		// layout — see internal/ui/motion's own package doc for why CSS
		// vs. hand-rolled animations are gated differently.
		if playing, left := justInserted(it.Born, time.Now()); playing && motion.EnableAnimations() {
			b.row.AddCSSClass("just-inserted")
			// GTK CSS has no animation-end signal, so without this timer
			// the class would replay on the row's next recycle.
			ms := uint(left/time.Millisecond) + 16
			b.strip = glib.TimeoutAdd(ms, func() bool {
				b.row.RemoveCSSClass("just-inserted")
				b.strip = 0
				return false
			})
		}
	}
	b.suppress = false
}

func (l *List) unbindRow(obj *coreglib.Object) {
	li := obj.Cast().(*gtk.ListItem)
	if b, ok := l.rows[li.Native()]; ok {
		b.cancelStrip()
		b.cancelFlash()
		b.item = nil
	}
}

func (l *List) teardownRow(obj *coreglib.Object) {
	li := obj.Cast().(*gtk.ListItem)
	if b, ok := l.rows[li.Native()]; ok {
		b.cancelStrip()
		b.cancelFlash()
	}
	delete(l.rows, li.Native())
}

// FlashDuplicate briefly pulses it's row ring twice over
// theme.DuplicateFlashDuration — the panel's response (copper-l2z.26) to
// a capture that duplicates the newest note. A no-op if it is not
// currently bound to a live (on-screen) row: an off-screen item has
// nothing to flash, and there is no persistent "pending flash" state to
// replay once it scrolls into view. A repeat call within the same
// window (it flashing again before the first flash finishes) restarts
// the timer but does not restart the CSS animation itself — the class
// is already present, so GTK never replays ingot-duplicate-flash — a
// rare-enough edge case (back-to-back duplicate captures of the exact
// same note) that it is left as a known limitation rather than adding a
// forced-reflow workaround.
func (l *List) FlashDuplicate(it *Item) {
	if it == nil {
		return
	}
	for _, b := range l.rows {
		if b.item != it {
			continue
		}
		b.cancelFlash()
		b.row.AddCSSClass("duplicate-flash")
		b.flash = glib.TimeoutAdd(theme.DuplicateFlashDuration, func() bool {
			b.row.RemoveCSSClass("duplicate-flash")
			b.flash = 0
			return false
		})
		return
	}
}

func (l *List) setupHeader(obj *coreglib.Object) {
	h := obj.Cast().(*gtk.ListHeader)
	sh := newSectionHeader()
	h.SetChild(sh)
	l.headers[h.Native()] = &headerBinding{header: sh}
}

func (l *List) bindHeader(obj *coreglib.Object) {
	h := obj.Cast().(*gtk.ListHeader)
	hb, ok := l.headers[h.Native()]
	if !ok {
		return
	}
	it := itemModel.ObjectValue(h.Item())
	hb.header.SetTitle(l.model.TitleOf(it.SectionID))
	if l.onHeaderBind != nil {
		l.onHeaderBind(h.Start(), h.NItems())
	}
}

func (l *List) teardownHeader(obj *coreglib.Object) {
	h := obj.Cast().(*gtk.ListHeader)
	delete(l.headers, h.Native())
}
