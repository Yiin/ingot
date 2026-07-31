// Package search drives the panel's live search field: on every
// keystroke it recomputes which sections and notes internal/ui/notelist
// should show, which raw-body byte ranges to highlight, and which note
// should hold keyboard focus, then pushes all three into a live
// notelist.List through a gtk.FilterListModel inserted between the
// notelist's item model and its sorter.
//
// Filtering semantics come from internal/store/searchtext — Tokens,
// Normalize, and Normalized.Match are the exact primitives
// internal/store/fsstore's own Search uses. This package does not
// reimplement that matching; it only adapts those primitives to a live
// GTK model addressed by *notelist.Item pointer, since
// internal/store/searchtext.Filter's positional-slice shape (parallel to
// a []store.Section) has no way to report back which live Item a given
// match belongs to.
package search
