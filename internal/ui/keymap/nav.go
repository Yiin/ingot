package keymap

// Row is the minimal read-only view of one displayed note-list row that
// Nav's focus and selection logic needs. Order matters: it must be the
// list's actual top-to-bottom display order, not insertion order — the
// real UI adapts notelist's sorted view onto this; tests use a plain
// fixture.
type Row struct {
	ID        string
	SectionID string
}

// Nav is the notelist's keyboard and mouse navigation and selection
// state: a focused row plus an anchor-and-extent selection, over a
// fixed display order. It has no GTK dependency, so it is testable with
// plain go test — a real key controller and click gesture drive it, and
// a GtkMultiSelection is synced to Selected() after every call.
type Nav struct {
	rows   []Row
	focus  int // display index of the focused row, -1 if none
	anchor int // display index a range selection extends from, -1 if none
	sel    map[string]bool
}

// NewNav returns a Nav over rows in display order, with nothing focused
// or selected.
func NewNav(rows []Row) *Nav {
	n := &Nav{focus: -1, anchor: -1, sel: make(map[string]bool)}
	n.rows = append([]Row(nil), rows...)
	return n
}

// SetRows replaces the display order — a reload, a filter change, or a
// resort. Focus and the anchor are kept by row ID where that ID still
// exists in the new order, cleared otherwise; any selected ID no longer
// present is dropped.
func (n *Nav) SetRows(rows []Row) {
	focusedID, anchorID := n.idAt(n.focus), n.idAt(n.anchor)
	n.rows = append([]Row(nil), rows...)
	n.focus = n.indexOf(focusedID)
	n.anchor = n.indexOf(anchorID)
	for id := range n.sel {
		if n.indexOf(id) < 0 {
			delete(n.sel, id)
		}
	}
}

func (n *Nav) idAt(i int) string {
	if i < 0 || i >= len(n.rows) {
		return ""
	}
	return n.rows[i].ID
}

func (n *Nav) indexOf(id string) int {
	if id == "" {
		return -1
	}
	for i, r := range n.rows {
		if r.ID == id {
			return i
		}
	}
	return -1
}

// Focus returns the focused row's display index, or -1 if none.
func (n *Nav) Focus() int { return n.focus }

// FocusedID returns the focused row's ID, or "" if none.
func (n *Nav) FocusedID() string { return n.idAt(n.focus) }

// Selected returns every selected row's ID, in display order.
func (n *Nav) Selected() []string {
	out := make([]string, 0, len(n.sel))
	for _, r := range n.rows {
		if n.sel[r.ID] {
			out = append(out, r.ID)
		}
	}
	return out
}

// IsSelected reports whether id is currently selected.
func (n *Nav) IsSelected(id string) bool { return n.sel[id] }

// HasSelection reports whether any row is selected — the Escape
// cascade's step 4 check.
func (n *Nav) HasSelection() bool { return len(n.sel) > 0 }

// selectOnly collapses the selection to exactly rows[i] (or to nothing,
// for i < 0) and moves both focus and the anchor there — the effect of
// every plain, non-extending focus move.
func (n *Nav) selectOnly(i int) {
	n.focus = i
	n.anchor = i
	n.sel = make(map[string]bool)
	if i >= 0 {
		n.sel[n.rows[i].ID] = true
	}
}

// FocusNext moves focus one row down (Down), collapsing the selection
// to the newly focused row. A no-op at the last row; focuses the first
// row if nothing was focused.
func (n *Nav) FocusNext() {
	if len(n.rows) == 0 {
		return
	}
	if n.focus < 0 {
		n.selectOnly(0)
		return
	}
	if n.focus >= len(n.rows)-1 {
		return
	}
	n.selectOnly(n.focus + 1)
}

// FocusPrevious moves focus one row up (Up), collapsing the selection.
// A no-op at the first row; focuses the first row if nothing was
// focused.
func (n *Nav) FocusPrevious() {
	if len(n.rows) == 0 {
		return
	}
	if n.focus < 0 {
		n.selectOnly(0)
		return
	}
	if n.focus <= 0 {
		return
	}
	n.selectOnly(n.focus - 1)
}

// FocusFirst moves focus to the first row (Home), collapsing the
// selection.
func (n *Nav) FocusFirst() {
	if len(n.rows) == 0 {
		return
	}
	n.selectOnly(0)
}

// FocusLast moves focus to the last row (End), collapsing the
// selection.
func (n *Nav) FocusLast() {
	if len(n.rows) == 0 {
		return
	}
	n.selectOnly(len(n.rows) - 1)
}

// JumpNextSection moves focus to the first row of the next section
// after the focused row's own section (Ctrl+Down), collapsing the
// selection. A no-op if the focused row is already in the last section,
// or nothing is focused.
func (n *Nav) JumpNextSection() {
	if n.focus < 0 {
		return
	}
	cur := n.rows[n.focus].SectionID
	for i := n.focus + 1; i < len(n.rows); i++ {
		if n.rows[i].SectionID != cur {
			n.selectOnly(i)
			return
		}
	}
}

// JumpPreviousSection moves focus to the first row of the previous
// section before the focused row's own section (Ctrl+Up), collapsing
// the selection. A no-op if the focused row is already in the first
// section, or nothing is focused.
func (n *Nav) JumpPreviousSection() {
	if n.focus < 0 {
		return
	}
	cur := n.rows[n.focus].SectionID
	start := n.focus
	for start > 0 && n.rows[start-1].SectionID == cur {
		start--
	}
	if start == 0 {
		return
	}
	prevSection := n.rows[start-1].SectionID
	prevStart := start - 1
	for prevStart > 0 && n.rows[prevStart-1].SectionID == prevSection {
		prevStart--
	}
	n.selectOnly(prevStart)
}

// ExtendDown extends the selection one row down from the anchor
// (Shift+Down): focus moves down, clamped at the last row, and the
// selection becomes the inclusive range between the anchor and the new
// focus. Behaves like FocusNext if nothing was focused.
func (n *Nav) ExtendDown() { n.extend(1) }

// ExtendUp extends the selection one row up from the anchor
// (Shift+Up). Behaves like FocusPrevious if nothing was focused.
func (n *Nav) ExtendUp() { n.extend(-1) }

func (n *Nav) extend(delta int) {
	if len(n.rows) == 0 {
		return
	}
	if n.focus < 0 {
		n.selectOnly(0)
		return
	}
	if n.anchor < 0 {
		n.anchor = n.focus
	}
	next := n.focus + delta
	if next < 0 || next >= len(n.rows) {
		return
	}
	n.focus = next
	n.selectRange(n.anchor, n.focus)
}

func (n *Nav) selectRange(a, b int) {
	if a > b {
		a, b = b, a
	}
	n.sel = make(map[string]bool, b-a+1)
	for i := a; i <= b; i++ {
		n.sel[n.rows[i].ID] = true
	}
}

// ToggleClick toggles row i's own selection (Ctrl+click) without
// disturbing any other row's selection, and moves both focus and the
// anchor to i so a following Shift+click ranges from here.
func (n *Nav) ToggleClick(i int) {
	if i < 0 || i >= len(n.rows) {
		return
	}
	id := n.rows[i].ID
	if n.sel[id] {
		delete(n.sel, id)
	} else {
		n.sel[id] = true
	}
	n.focus = i
	n.anchor = i
}

// RangeClick selects the inclusive range between the anchor and i
// (Shift+click), replacing the current selection. If there is no
// anchor yet, it is set to i, making this a single-row select. Focus
// moves to i; the anchor itself is unchanged, so a further Shift+click
// re-ranges from the same start.
func (n *Nav) RangeClick(i int) {
	if i < 0 || i >= len(n.rows) {
		return
	}
	if n.anchor < 0 {
		n.anchor = i
	}
	n.focus = i
	n.selectRange(n.anchor, i)
}

// SelectAllInSection selects every row sharing the focused row's
// SectionID (Ctrl+A with the list focused — a focused text field must
// never reach this, see ShouldGateForList). A no-op if nothing is
// focused. Focus and the anchor are left where they were.
func (n *Nav) SelectAllInSection() {
	if n.focus < 0 {
		return
	}
	id := n.rows[n.focus].SectionID
	for _, r := range n.rows {
		if r.SectionID == id {
			n.sel[r.ID] = true
		}
	}
}

// ClearSelection empties the selection without moving focus — the
// Escape cascade's step 4.
func (n *Nav) ClearSelection() {
	n.sel = make(map[string]bool)
}

// SyncFocus moves focus and the anchor to id without touching the
// current selection — for keeping Nav's own row-order state aligned
// with a selection change driven outside Nav (the real widget's own
// default mouse-click handling, which this package's key controller
// deliberately leaves alone — see InstallNav's doc comment), so a
// following keyboard move continues from the row the mouse actually
// last touched instead of wherever Nav's keyboard focus happened to be.
// A no-op if id is not present in the current row order.
func (n *Nav) SyncFocus(id string) {
	i := n.indexOf(id)
	if i < 0 {
		return
	}
	n.focus = i
	n.anchor = i
}

// SyncSelection replaces the current selection with ids (any id not
// present in the current row order is silently dropped, same as
// SetRows already does for a selection that outlives its row) and, if
// ids is non-empty, syncs focus to the last one via SyncFocus — the
// selection counterpart to SyncFocus, for the same externally-driven-
// change reason: without this, Nav's own selection map only ever
// reflects what Nav itself last computed, so a mouse click — real
// selection changes, Nav's map does not — leaves Nav's very next
// keyboard-driven extend/select-all operating on a stale base set
// instead of what's actually selected on screen.
func (n *Nav) SyncSelection(ids []string) {
	n.sel = make(map[string]bool, len(ids))
	for _, id := range ids {
		if n.indexOf(id) >= 0 {
			n.sel[id] = true
		}
	}
	if len(ids) > 0 {
		n.SyncFocus(ids[len(ids)-1])
	}
}
