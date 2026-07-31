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
	li   *gtk.ListItem
	row  *widget.Row
	clip *gtk.Revealer // wraps row; lets growRowIn animate row's own AllocatedHeight in from 0 (see growRowIn)
	ph   *gtk.Label    // "No notes yet" placeholder card; exactly one of row/ph is visible

	item     *Item
	suppress bool // true while Bind is driving Row's setters programmatically
	strip    glib.SourceHandle
	flash    glib.SourceHandle
	growIdle glib.SourceHandle // pending growRowIn deferral; see growRowIn's own doc comment
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

func (b *rowBinding) cancelGrowIdle() {
	if b.growIdle != 0 {
		glib.SourceRemove(b.growIdle)
		b.growIdle = 0
	}
}

// resetGrow collapses clip instantly (no transition) — the baseline every
// bind starts from before growRowIn (or an instant re-reveal, for an
// already-settled item) decides the real state. GtkRevealer has no
// "cancel an in-flight transition" call of its own; retargeting
// SetRevealChild mid-transition is enough (it does not need an explicit
// stop first). It does cancel a still-pending growRowIn deferral, the one
// piece of grow-related state that does need explicit teardown.
func (b *rowBinding) resetGrow() {
	b.cancelGrowIdle()
	b.clip.SetTransitionDuration(0)
	b.clip.SetRevealChild(false)
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
	// rowWidgets indexes the same live bindings as rows, but by the row
	// widget's own GObject identity (coreglib.BaseObject(b.row).Native())
	// rather than the recycled ListItem's — RowAt walks up from
	// gtk.Widget.Pick's result through the widget's ancestors, which
	// passes through the row widget itself, never the private
	// GtkListItemWidget wrapper GTK parents it under, so this is the map
	// that lookup actually needs.
	rowWidgets map[uintptr]*rowBinding

	anchor *Item

	onToggle           func(it *Item, done bool)
	onSelectionChanged func()
	onActivate         func(it *Item)
	onEditCommitted    func(id, text string)

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
		model:      m,
		rows:       make(map[uintptr]*rowBinding),
		headers:    make(map[uintptr]*headerBinding),
		rowWidgets: make(map[uintptr]*rowBinding),
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
		if pos := l.ViewPositionOf(it); pos >= 0 {
			l.sel.SelectItem(uint(pos), false)
		}
	}
}

// ViewLen returns the number of rows currently in sorted (displayed)
// order, including any placeholder — the same coordinate space
// ItemAtViewPosition and RowAt use.
func (l *List) ViewLen() int { return int(l.sel.NItems()) }

// ViewPositionOf returns it's position in l.sel — the list's actual
// current displayed order — or -1 if it is not currently visible
// (filtered out by SetFilter, or not in the model at all).
//
// This is deliberately NOT Model.ViewPosition: that method computes a
// position over the model's full, unfiltered base order, blind to
// whatever predicate SetFilter currently has installed. With no filter
// active the two agree, but with one active they diverge — Model.
// ViewPosition would return an index that means something different in
// l.sel's own (filtered) numbering, silently pointing every caller here
// at the wrong row. A linear scan is the only way to answer this
// correctly against the live GtkSelectionModel; it costs the same O(n)
// Model.ViewPosition's own linear scan already does, over the same
// (recycled-widget-bounded, not model-sized) live count in practice.
func (l *List) ViewPositionOf(it *Item) int {
	n := l.sel.NItems()
	for i := uint(0); i < n; i++ {
		if itemModel.ObjectValue(l.sel.Item(i)) == it {
			return int(i)
		}
	}
	return -1
}

// ItemAtViewPosition returns the real note at sorted (displayed)
// position pos — the inverse of ViewPositionOf — or nil if pos is out
// of range or names a placeholder. Positions come from RowAt or from a
// caller's own iteration over the live GtkSelectionModel; both agree
// with ViewPositionOf's coordinate space, since all three read the same
// l.sel.
func (l *List) ItemAtViewPosition(pos int) *Item {
	if pos < 0 || uint(pos) >= l.sel.NItems() {
		return nil
	}
	it := itemModel.ObjectValue(l.sel.Item(uint(pos)))
	if it.IsPlaceholder() {
		return nil
	}
	return it
}

// RowAt implements menus.RowLocator: it translates a pixel position
// relative to the ListView (ListView, not List's own outer Overlay —
// the caller attaches the right-click gesture there so these
// coordinates line up with no scrollbar/overlay offset to account for)
// into the row displayed at that point and its own sorted-order index,
// via gtk.Widget.Pick plus an ancestor walk: Pick finds the topmost
// widget actually rendered under the point, which is some descendant of
// one recycled row's own widget.Row (its checkbox, its label, ...), not
// the row itself, and never the private GtkListItemWidget GTK parents
// rows under, which carries no identity this package can look up rows
// by — so the walk climbs from whatever was hit until it reaches a
// widget this package tracks in rowWidgets, or reaches the ListView
// itself with no match (a click that landed between rows, or nowhere).
func (l *List) RowAt(x, y float64) (row gtk.Widgetter, index int, ok bool) {
	picked := l.listView.Widget.Pick(x, y, gtk.PickDefault)
	lvNative := coreglib.BaseObject(l.listView).Native()

	for w := picked; w != nil; w = gtk.BaseWidget(w).Parent() {
		native := coreglib.BaseObject(w).Native()
		if native == lvNative {
			break
		}
		b, tracked := l.rowWidgets[native]
		if !tracked {
			continue
		}
		if b.item == nil || b.item.IsPlaceholder() {
			return nil, 0, false
		}
		pos := l.ViewPositionOf(b.item)
		if pos < 0 {
			return nil, 0, false
		}
		return b.row, pos, true
	}
	return nil, 0, false
}

// IsTruncatedAt reports whether the row currently displayed at sorted
// position pos is showing an ellipsis, per widget.Label.IsTruncated —
// false for a position with no currently bound (on-screen) row, since
// truncation is only meaningful once a label has been allocated.
func (l *List) IsTruncatedAt(pos int) bool {
	it := l.ItemAtViewPosition(pos)
	if it == nil {
		return false
	}
	for _, b := range l.rows {
		if b.item == it {
			return b.row.Label.IsTruncated()
		}
	}
	return false
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
	if pos := l.ViewPositionOf(it); pos >= 0 {
		l.listView.ScrollTo(uint(pos), gtk.ListScrollNone, nil)
	}
}

// ScrollToAndSelect scrolls to and selects it — the typical response to a
// fresh capture landing at the top of its section.
func (l *List) ScrollToAndSelect(it *Item) {
	if pos := l.ViewPositionOf(it); pos >= 0 {
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

// ConnectEditCommitted registers f to run whenever an inline edit
// started by StartInlineEdit commits (Enter), with the edited note's ID
// and its new trimmed body.
func (l *List) ConnectEditCommitted(f func(id, text string)) { l.onEditCommitted = f }

// boundRow returns the rowBinding currently displaying it, or nil if it
// has no live (on-screen) row — the same off-screen-is-a-no-op contract
// as FlashDuplicate below.
func (l *List) boundRow(it *Item) *rowBinding {
	for _, b := range l.rows {
		if b.item == it {
			return b
		}
	}
	return nil
}

// StartInlineEdit swaps the row displaying id's note for an inline
// composer.Composer seeded with its raw Markdown body — see
// widget.Row.StartEdit. Committing (Enter) updates the item's Body,
// re-renders the label, and fires ConnectEditCommitted. A no-op if id
// names no item, or that item has no currently-bound (on-screen) row.
func (l *List) StartInlineEdit(id string) {
	it := l.model.ItemByID(id)
	if it == nil {
		return
	}
	b := l.boundRow(it)
	if b == nil {
		return
	}
	b.row.StartEdit(it.Body, func(text string) {
		it.Body = text
		b.row.Label.SetBody(text)
		// SetBody always renders the clamped, single-paragraph markup —
		// if the row was expanded before editing started, re-apply that
		// so committing does not silently collapse it back to 3 lines
		// while the .expanded CSS class (and IsExpanded()) still claims
		// otherwise.
		if b.row.IsExpanded() {
			b.row.Label.Expand()
		}
		if l.onEditCommitted != nil {
			l.onEditCommitted(it.ID, text)
		}
	})
}

// SetExpanded drops or restores id's row's 3-line cap — see
// widget.Row.SetExpanded. A no-op if id names no item, or that item has
// no currently-bound (on-screen) row.
func (l *List) SetExpanded(id string, expanded bool) {
	it := l.model.ItemByID(id)
	if it == nil {
		return
	}
	if b := l.boundRow(it); b != nil {
		b.row.SetExpanded(expanded)
	}
}

// ToggleExpanded flips id's row between collapsed and expanded — Alt+
// Enter's action. A no-op if id names no item, or that item has no
// currently-bound (on-screen) row.
func (l *List) ToggleExpanded(id string) {
	it := l.model.ItemByID(id)
	if it == nil {
		return
	}
	if b := l.boundRow(it); b != nil {
		b.row.SetExpanded(!b.row.IsExpanded())
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

	// clip wraps row unconditionally, not just while animating, so
	// growRowIn never has to reparent anything mid-bind. GtkRevealer, not
	// a GtkScrolledWindow: GTK's CSS engine does not interpolate
	// min-height/padding/margin (measured live, copper-5g4), and a
	// ScrolledWindow's own MinContentHeight/MaxContentHeight turned out
	// not to support a smooth intermediate clip either (also measured
	// live: forcing both to the same ticking value snapped straight from
	// 0 to the child's full natural size on the first nonzero tick,
	// instead of growing through it). GtkRevealer's slide transition is a
	// real GTK-native animation built for exactly this "allocate less
	// than natural, growing over time" case, and GtkListView's row
	// measurement respects it correctly.
	clip := gtk.NewRevealer()
	clip.SetChild(row)
	clip.SetTransitionType(gtk.RevealerTransitionTypeSlideDown)
	clip.SetRevealChild(true)

	ph := gtk.NewLabel("No notes yet")
	ph.AddCSSClass("note-placeholder")
	ph.SetHAlign(gtk.AlignFill)
	ph.SetVAlign(gtk.AlignCenter)

	wrap := gtk.NewBox(gtk.OrientationVertical, 0)
	wrap.Append(clip)
	wrap.Append(ph)
	li.SetChild(wrap)

	b := &rowBinding{li: li, row: row, clip: clip, ph: ph}
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
	l.rowWidgets[coreglib.BaseObject(row).Native()] = b
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
	b.resetGrow() // first, unconditionally — see resetGrow's own doc comment

	b.suppress = true
	b.row.RemoveCSSClass("just-inserted")   // first, unconditionally
	b.row.RemoveCSSClass("duplicate-flash") // same: never replay on recycle

	placeholder := it.IsPlaceholder()
	b.row.SetVisible(!placeholder)
	b.clip.SetVisible(!placeholder)
	b.ph.SetVisible(placeholder)
	// A placeholder is not a note: keep it out of GtkSelectionModel's
	// selection and activation entirely, not just filtered out of this
	// package's own Selected()/activate helpers — otherwise Ctrl+A or a
	// rubber-band selection can select "nothing" at the model level.
	li.SetSelectable(!placeholder)
	li.SetActivatable(!placeholder)

	if !placeholder {
		b.row.CancelEdit()
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
		// vs. hand-rolled animations are gated differently. growRowIn is
		// called unconditionally either way — motion.Reveal, underneath
		// it, already honours EnableAnimations() for free (GtkRevealer's
		// transition is a built-in GTK animation, not a hand-rolled one).
		if playing, left := justInserted(it.Born, time.Now()); playing {
			if motion.EnableAnimations() {
				b.row.AddCSSClass("just-inserted")
				// GTK CSS has no animation-end signal, so without this
				// timer the class would replay on the row's next recycle.
				ms := uint(left/time.Millisecond) + 16
				b.strip = glib.TimeoutAdd(ms, func() bool {
					b.row.RemoveCSSClass("just-inserted")
					b.strip = 0
					return false
				})
			}
			growRowIn(b, left)
		} else {
			// Not a fresh insert: reveal instantly, no transition —
			// resetGrow above only ever collapses; every real item must
			// end this function actually visible.
			b.clip.SetTransitionDuration(0)
			b.clip.SetRevealChild(true)
		}
	}
	b.suppress = false
}

// growRowIn animates b.row's own AllocatedHeight from 0 up to its natural
// size over left (the remaining slice of InsertAnimDuration — the same
// value the opacity fade's own timer above uses, so a row rebound
// mid-flight onto a still-fresh item resumes rather than replays).
//
// style.css's ingot-row-in @keyframes handles opacity for free — GTK's
// CSS engine interpolates that fine — but it cannot animate min-height/
// padding/margin-top the same way (measured live, copper-5g4: sampled for
// 300ms, no growth at all). A GtkScrolledWindow's own MinContentHeight/
// MaxContentHeight, hand-ticked, was the first thing tried instead and
// also measured live to not work: GtkListView never allocates a plain
// widget smaller than its natural minimum regardless of SetSizeRequest,
// and forcing a ScrolledWindow's own min/max content height to the same
// ticking value did shrink it correctly at 0, but on any nonzero tick
// snapped straight to the child's full natural size instead of growing
// through the intermediate values. b.clip is a GtkRevealer instead: its
// slide transition is a real GTK-native animation built for exactly this
// "allocate less than natural, growing over time" case, and needs no
// measured target at all — GTK drives it against the child's own natural
// size directly.
//
// The SetRevealChild(true) call is deferred one main-loop turn via
// IdleAdd — measured live, calling it in the same call stack that just
// collapsed the row (resetGrow, called earlier in this same bindRow) and
// created/bound the row's own widgets never actually animates: GTK
// treats a reveal-child flip with no intervening frame as the widget's
// initial state, not a transition, and jumps straight to fully revealed.
// Giving resetGrow's collapsed state one real frame first is what makes
// the following reveal a genuine, observable state change. growIdle
// tracks the pending callback so a row recycled before it fires (a new
// bind, or teardown) can cancel it — see resetGrow and cancelGrowIdle.
func growRowIn(b *rowBinding, left time.Duration) {
	it := b.item
	b.growIdle = glib.IdleAdd(func() bool {
		b.growIdle = 0
		if b.item == it { // not rebound to something else while this was pending
			motion.Reveal(b.clip, true, left, left)
		}
		return false
	})
}

func (l *List) unbindRow(obj *coreglib.Object) {
	li := obj.Cast().(*gtk.ListItem)
	if b, ok := l.rows[li.Native()]; ok {
		b.cancelStrip()
		b.cancelFlash()
		b.cancelGrowIdle()
		b.row.CancelEdit() // don't let a pooled row hold a live composer until its next bind
		b.item = nil
	}
}

func (l *List) teardownRow(obj *coreglib.Object) {
	li := obj.Cast().(*gtk.ListItem)
	if b, ok := l.rows[li.Native()]; ok {
		b.cancelStrip()
		b.cancelFlash()
		delete(l.rowWidgets, coreglib.BaseObject(b.row).Native())
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
