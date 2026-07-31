package notelist

import "time"

// InsertAnimDuration is the just-inserted row animation's length, and must
// stay equal to the duration named in style.css's
// ".note-card.just-inserted { animation: ingot-row-in <duration> ... }" —
// see TestInsertAnimDurationMatchesCSS.
const InsertAnimDuration = 180 * time.Millisecond

// Kind distinguishes a real note item from a section's empty-state
// placeholder in the underlying model. It is unexported so callers cannot
// forge a placeholder — Model creates and destroys them itself.
type kind uint8

const (
	kindNote kind = iota
	kindPlaceholder
)

// Item is one row's worth of data: either a real note or, when its
// section has zero notes, that section's own "No notes yet" placeholder
// card. Item is always handled by pointer — pointer identity is item
// identity throughout this package (selection, ordering, the birth
// timestamp survive a Splice because the Go value does, even though the
// gioutil model reboxes it).
type Item struct {
	// ID is the caller-owned stable identifier (a future store.Note.ID).
	// Empty for a placeholder.
	ID string
	// Body is the note's Markdown body, fed to widget.Label.SetBody.
	Body string
	// Done is the note's checked state.
	Done bool
	// SectionID names the Section this item belongs to.
	SectionID string
	// Born is the item's insertion timestamp, kept on the model value
	// (not the recycled row widget) because ConnectBind needs it on
	// every rebind to decide whether to replay the insert animation.
	Born time.Time
	// Ranges is the note's active search-match highlight — raw-body
	// [start, end) byte offsets into Body, in the same coordinates as
	// searchtext's Hit.Ranges/NoteFilter.Ranges. Set by
	// internal/ui/search on every query change; empty outside an active
	// search. bindRow pushes it onto the bound row's Label via
	// SetHighlight on every bind, and List.RefreshHighlights does the
	// same for a row that stays bound across a query change.
	Ranges [][2]int

	kind kind
	seq  int64 // intra-section order key, assigned by Model
}

// NewItem returns a new, freshly-born real note Item.
func NewItem(id, sectionID, body string, done bool) *Item {
	return &Item{
		ID:        id,
		Body:      body,
		Done:      done,
		SectionID: sectionID,
		Born:      time.Now(),
		kind:      kindNote,
	}
}

// IsPlaceholder reports whether it is a section's own empty-state card
// rather than a real note.
func (it *Item) IsPlaceholder() bool { return it.kind == kindPlaceholder }

// Section is one named group of items, declared up front in display
// order — that order is the sort rank sections are shown in (see
// order.go), never alphabetical.
type Section struct {
	ID    string
	Title string
}

// justInserted reports whether an item born at t should still carry the
// .just-inserted CSS class at now, and if so how much animation time
// remains — the pure half of "a re-bound old note never carries the
// just-inserted class".
func justInserted(born, now time.Time) (bool, time.Duration) {
	left := InsertAnimDuration - now.Sub(born)
	if left <= 0 {
		return false, 0
	}
	return true, left
}
