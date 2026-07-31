package notelist

import (
	"unsafe"

	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

// itemModel is the package-level gioutil type token for *Item. gotk4 v0.4.0
// has glib.RegisterSubclass, but pkg/core/gioutil already ships a C-side
// GListModel (Gotk4GboxList) storing arbitrary Go values through the gbox
// registry, and it is GC-safe — subclassing GObject from Go is not needed
// and must not be used here.
var itemModel = gioutil.NewListModelType[*Item]()

// rankOf returns id's position in the declared section order, or
// len(sections) — one trailing bucket — for an undeclared id. That never
// panics on a caller bug (an Item.SectionID that doesn't match any
// declared Section); it just sorts last, degrading instead of crashing.
func (m *Model) rankOf(sectionID string) int {
	if r, ok := m.rank[sectionID]; ok {
		return r
	}
	return len(m.sections)
}

// compareSection is the GtkSortListModel section sorter: a rank
// difference, never a string compare — so sections land in the project's
// declared order, not alphabetically — and it returns 0 for any two items
// in the same section, which is exactly what makes a section a maximal
// run of equal items to GtkSortListModel. Feeding this the seq tiebreak
// too would mean no two items ever compare equal, and the header factory
// would fire once per item instead of once per section.
func (m *Model) compareSection(a, b *Item) int {
	return m.rankOf(a.SectionID) - m.rankOf(b.SectionID)
}

// compareOrder is the GtkSortListModel item sorter: compareSection's rank
// difference first, then the intra-section seq tiebreak Model maintains.
// It is a total order over the base slice, so the displayed order never
// depends on GtkSortListModel's incremental sort being stable across a
// splice — which matters because InsertAt and Move only mean anything if
// a freshly inserted item reliably lands at its intended position.
func (m *Model) compareOrder(a, b *Item) int {
	if d := m.compareSection(a, b); d != 0 {
		return d
	}
	switch {
	case a.seq < b.seq:
		return -1
	case a.seq > b.seq:
		return 1
	default:
		return 0
	}
}

// sorterFunc wraps a *Item comparator as the raw-pointer glib.CompareDataFunc
// gtk.NewCustomSorter needs. coreglib.Take borrows the object (transfer
// none) so ObjectValue can read the boxed *Item back out through the gbox
// registry.
func sorterFunc(cmp func(a, b *Item) int) glib.CompareDataFunc {
	return func(a, b unsafe.Pointer) int {
		if a == nil || b == nil {
			return 0
		}
		ia := itemModel.ObjectValue(coreglib.Take(a))
		ib := itemModel.ObjectValue(coreglib.Take(b))
		return cmp(ia, ib)
	}
}
