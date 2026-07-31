package notelist

import "github.com/diamondburned/gotk4/pkg/core/gioutil"

// Model owns the notelist's data: the declared sections, every item in
// base (insertion) order, and the gioutil-boxed GListModel that mirrors
// that same order into GTK. It does its own bookkeeping in plain Go — no
// display, no windowing — so it is testable with plain `go test`.
//
// Invariant, maintained after every mutation: for each declared section,
// the base slice contains exactly one placeholder Item iff that section
// currently has zero real-note items. This is what makes an empty
// section still render a header, rule and "No notes yet" card — a plain
// GtkSectionModel only emits a header for a section that actually
// contains model items.
type Model struct {
	gl *gioutil.ListModel[*Item]

	sections []Section
	rank     map[string]int // Section.ID -> declared order
	title    map[string]string
	counts   map[string]int   // real-note count per Section.ID
	holder   map[string]*Item // live placeholder per empty Section.ID

	list []*Item // base order, mirrors gl 1:1

	nextSeq int64

	// invalidate is called whenever a mutation could have changed the
	// relative order of items that were already in the model (as
	// opposed to a plain Append, whose fresh, always-larger seq needs
	// no resort notification). Set by List to notify the order sorter;
	// nil-safe so Model is usable standalone in tests.
	invalidate func()
}

// NewModel returns a Model over sections, in the given display order,
// each starting empty (and so each carrying its own placeholder).
func NewModel(sections []Section) *Model {
	m := &Model{
		gl:     itemModel.New(),
		counts: make(map[string]int),
		holder: make(map[string]*Item),
	}
	m.SetSections(sections)
	return m
}

// ListModel returns the underlying gioutil model, for List to wire into
// the GtkSortListModel.
func (m *Model) ListModel() *gioutil.ListModel[*Item] { return m.gl }

// Sections returns the currently declared sections, in display order.
func (m *Model) Sections() []Section {
	out := make([]Section, len(m.sections))
	copy(out, m.sections)
	return out
}

// TitleOf returns the display title for a declared section, or the
// uppercased id itself for a SectionID an Item carries that was never
// declared — a programming error, but one that degrades instead of
// panicking (see rankOf).
func (m *Model) TitleOf(sectionID string) string {
	if t, ok := m.title[sectionID]; ok {
		return t
	}
	return sectionID
}

// SetSections replaces the declared sections (and their order), rebuilding
// every placeholder to match. Existing real items whose SectionID is no
// longer declared keep their data but sort into the trailing "undeclared"
// bucket (see rankOf) until reassigned.
func (m *Model) SetSections(sections []Section) {
	m.sections = append([]Section(nil), sections...)
	m.rank = make(map[string]int, len(sections))
	m.title = make(map[string]string, len(sections))
	for i, s := range sections {
		m.rank[s.ID] = i
		m.title[s.ID] = s.Title
		if _, ok := m.counts[s.ID]; !ok {
			m.counts[s.ID] = 0
		}
	}

	// A placeholder for a section that is no longer declared must go
	// even though sync's own removal pass only fires once that section's
	// count is > 0 — a count that will never come, for a section that no
	// longer exists at all.
	for id, ph := range m.holder {
		if _, declared := m.rank[id]; !declared {
			if i := m.IndexOf(ph); i >= 0 {
				m.list = append(m.list[:i], m.list[i+1:]...)
				m.gl.Splice(i, 1)
			}
			delete(m.holder, id)
		}
	}

	m.sync()
	m.notifyInvalidate()
}

// Len returns the number of items in base order, including placeholders.
func (m *Model) Len() int { return len(m.list) }

// At returns the item at base position i.
func (m *Model) At(i int) *Item { return m.list[i] }

// IndexOf returns it's base position, or -1 if it is not in the model.
func (m *Model) IndexOf(it *Item) int {
	for i, v := range m.list {
		if v == it {
			return i
		}
	}
	return -1
}

// Items returns every real (non-placeholder) note, in base order.
func (m *Model) Items() []*Item {
	out := make([]*Item, 0, len(m.list))
	for _, it := range m.list {
		if !it.IsPlaceholder() {
			out = append(out, it)
		}
	}
	return out
}

// Append adds it to the end of the base order.
func (m *Model) Append(it *Item) {
	m.assignSeq(it)
	m.list = append(m.list, it)
	m.gl.Append(it)
	m.bumpCount(it, 1)
	// A fresh, always-larger seq needs no resort notification: it can
	// only ever sort after every existing item in its section.
}

// AppendAll adds items to the end of the base order in one Splice — the
// 5000-items-in-5.3ms path.
func (m *Model) AppendAll(items []*Item) {
	if len(items) == 0 {
		return
	}
	for _, it := range items {
		m.assignSeq(it)
		if !it.IsPlaceholder() {
			m.counts[it.SectionID]++
		}
	}
	// list/gl are fully updated before sync() runs, so it never observes
	// an item that is counted but not yet present in either.
	m.list = append(m.list, items...)
	m.gl.Splice(m.gl.Len(), 0, items...)
	m.sync()
}

// InsertAt inserts it at base position i, renumbering every later item's
// seq so within-section order matches the new base order.
func (m *Model) InsertAt(i int, it *Item) {
	m.list = append(m.list, nil)
	copy(m.list[i+1:], m.list[i:])
	m.list[i] = it
	m.reseq()
	m.gl.Splice(i, 0, it)
	m.bumpCount(it, 1)
	m.notifyInvalidate()
}

// RemoveAt removes the item at base position i.
func (m *Model) RemoveAt(i int) {
	it := m.list[i]
	m.list = append(m.list[:i], m.list[i+1:]...)
	m.gl.Splice(i, 1)
	m.bumpCount(it, -1)
	// Removing never reorders survivors relative to each other.
}

// Move relocates the item at base position from so that it ends up at
// base position to (i.e. to is the item's own final index, in the same
// index space as At/IndexOf — not a pre-removal insertion gap), as two
// Splice calls (remove, then insert) per the child spec. Splicing an
// item removes and recreates its underlying gbox object, so GTK will see
// this as a remove+add — any live selection on that row is lost.
func (m *Model) Move(from, to int) {
	if from == to {
		return
	}
	it := m.list[from]
	m.list = append(m.list[:from], m.list[from+1:]...)
	m.gl.Splice(from, 1)

	// After removing from, the (n-1)-length list already has the
	// post-removal shift baked in, so inserting at to here — with no
	// further adjustment in either direction — lands it at index to in
	// the final n-length list.
	m.list = append(m.list, nil)
	copy(m.list[to+1:], m.list[to:])
	m.list[to] = it
	m.gl.Splice(to, 0, it)

	m.reseq()
	m.notifyInvalidate()
}

// Reset replaces every item (placeholders excluded — sync rebuilds them)
// in one Splice, for an initial load or a search-filtered view swap.
func (m *Model) Reset(items []*Item) {
	m.gl.Splice(0, m.gl.Len())
	m.list = nil
	m.holder = make(map[string]*Item)
	m.counts = make(map[string]int, len(m.sections))
	for _, s := range m.sections {
		m.counts[s.ID] = 0
	}

	// Populate list/gl and counts fully before sync() runs, so it never
	// observes a state where an item is counted but not yet present in
	// either slice.
	cp := append([]*Item(nil), items...)
	for _, it := range cp {
		m.assignSeq(it)
		if !it.IsPlaceholder() {
			m.counts[it.SectionID]++
		}
	}
	m.list = cp
	if len(cp) > 0 {
		m.gl.Splice(0, 0, cp...)
	}

	m.sync()
	m.notifyInvalidate()
}

// ViewPosition returns it's position in sorted (displayed) order, or -1
// if it is not in the model. It is a pure Go computation, independent of
// any live GtkSortListModel, computed as the count of every other item
// that sorts strictly before it (compareOrder is a strict total order —
// no two items share a (section, seq) pair — so this always agrees with
// a reference stable sort), so tests can check it against one without a
// display.
func (m *Model) ViewPosition(it *Item) int {
	if m.IndexOf(it) < 0 {
		return -1
	}
	pos := 0
	for _, v := range m.list {
		if v == it {
			continue
		}
		if m.compareOrder(v, it) < 0 {
			pos++
		}
	}
	return pos
}

// assignSeq gives it the next intra-section order key.
func (m *Model) assignSeq(it *Item) {
	it.seq = m.nextSeq
	m.nextSeq++
}

// reseq renumbers seq over the base slice, in order, so an InsertAt or
// Move is reflected in within-section display order too — without this,
// GtkSortListModel's stable sort could leave a freshly inserted item at
// the bottom of its section instead of where it was inserted.
func (m *Model) reseq() {
	for i, it := range m.list {
		it.seq = int64(i)
	}
	m.nextSeq = int64(len(m.list))
}

// bumpCount adjusts the real-note count for it's section (placeholders
// are not counted) and resyncs the placeholder invariant.
func (m *Model) bumpCount(it *Item, delta int) {
	if !it.IsPlaceholder() {
		m.counts[it.SectionID] += delta
	}
	// Always resync, even for a placeholder: RemoveAt can be called on a
	// placeholder's own base index (callers aren't required to filter
	// them out first), and without this the invariant — exactly one
	// placeholder per empty section — would stay broken until some
	// unrelated later mutation happened to call sync() again.
	m.sync()
}

// sync enforces the placeholder invariant: exactly one placeholder per
// empty declared section, none otherwise. Removals run before additions
// so a placeholder is never briefly adjacent to a real item in the same
// section.
func (m *Model) sync() {
	for id, ph := range m.holder {
		if m.counts[id] > 0 {
			if i := m.IndexOf(ph); i >= 0 {
				m.list = append(m.list[:i], m.list[i+1:]...)
				m.gl.Splice(i, 1)
			}
			delete(m.holder, id)
		}
	}
	for _, s := range m.sections {
		if m.counts[s.ID] == 0 {
			if ph, ok := m.holder[s.ID]; ok && m.IndexOf(ph) >= 0 {
				continue
			}
			ph := &Item{SectionID: s.ID, kind: kindPlaceholder}
			m.assignSeq(ph)
			m.holder[s.ID] = ph
			m.list = append(m.list, ph)
			m.gl.Append(ph)
		}
	}
}

func (m *Model) notifyInvalidate() {
	if m.invalidate != nil {
		m.invalidate()
	}
}
