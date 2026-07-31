package panel

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// state names the mutually exclusive layer the list area shows: the real
// note list, the first-run hint, or the search-no-matches block.
type state uint8

const (
	stateNormal state = iota
	stateHint
	stateSearchEmpty
)

// decideState is RefreshEmptyState's pure decision step, split out so it
// is unit-testable without a GTK display: "never show a truly empty
// shell" (stateHint, whenever there is not one real note anywhere) loses
// to "search with no matches" (stateSearchEmpty) whenever a non-empty
// query is actively producing zero results — a project can be genuinely
// non-empty and still have nothing to show for the current query.
func decideState(hasNotes bool, query string, matchCount int) state {
	switch {
	case query != "" && matchCount == 0:
		return stateSearchEmpty
	case !hasNotes:
		return stateHint
	default:
		return stateNormal
	}
}

// searchEmptyText is the search-no-matches block's label text, split out
// for the same reason as decideState.
func searchEmptyText(query string) string {
	return fmt.Sprintf("No notes match %q", query)
}

// newHintBlock builds the first-run "never show a truly empty shell"
// hint: title over subtitle, both --ink-muted via .panel-hint's
// inherited colour, centred with 40dp of breathing room. Starts hidden;
// RefreshEmptyState shows it.
func newHintBlock() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.AddCSSClass("panel-hint")
	box.SetHAlign(gtk.AlignCenter)
	box.SetVAlign(gtk.AlignCenter)
	box.SetCanTarget(false)
	box.SetVisible(false)

	title := gtk.NewLabel("Press Shift twice to capture")
	title.SetJustify(gtk.JustifyCenter)
	title.SetWrap(true)

	subtitle := gtk.NewLabel("Or type below to add a note.")
	subtitle.SetJustify(gtk.JustifyCenter)
	subtitle.SetWrap(true)

	box.Append(title)
	box.Append(subtitle)
	return box
}

// newSearchEmptyBlock builds the search-no-matches block: the query
// message in --ink over a "Clear search" text button (accent, no fill).
// onClear fires when that button is clicked. Starts hidden;
// RefreshEmptyState shows it and sets the label's query text.
func newSearchEmptyBlock(onClear func()) (*gtk.Box, *gtk.Label) {
	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.AddCSSClass("search-empty")
	box.SetHAlign(gtk.AlignCenter)
	box.SetVAlign(gtk.AlignCenter)
	box.SetVisible(false)

	label := gtk.NewLabel("")
	label.SetJustify(gtk.JustifyCenter)
	label.SetWrap(true)
	label.SetCanTarget(false)

	clear := gtk.NewButtonWithLabel("Clear search")
	clear.AddCSSClass("flat")
	clear.AddCSSClass("search-empty-clear")
	clear.SetHAlign(gtk.AlignCenter)
	clear.ConnectClicked(func() {
		if onClear != nil {
			onClear()
		}
	})

	box.Append(label)
	box.Append(clear)
	return box, label
}

// RefreshEmptyState recomputes which layer the list area shows, given
// the current search query and match count. Shell runs no search of its
// own: matchCount's source of truth is the caller — a fixture in a test
// today, copper-l2z.28's live filtering tomorrow, calling this every
// time it recomputes matches (mirroring internal/ui/searchbar's own
// SetMatchCount contract).
func (s *Shell) RefreshEmptyState(query string, matchCount int) {
	hasNotes := len(s.list.Model().Items()) > 0
	next := decideState(hasNotes, query, matchCount)
	s.lastState = next

	switch next {
	case stateSearchEmpty:
		s.list.ListView().SetVisible(false)
		s.hint.SetVisible(false)
		s.searchEmptyLabel.SetText(searchEmptyText(query))
		s.searchEmpty.SetVisible(true)
	case stateHint:
		// Hide the list too, not just overlay the hint on top of it: a
		// genuinely empty project is every declared section showing its
		// own "No notes yet" placeholder card, which would double up
		// with the hint's own "Press Shift twice to capture" message —
		// "never show a truly empty shell" means neither.
		s.list.ListView().SetVisible(false)
		s.searchEmpty.SetVisible(false)
		s.hint.SetVisible(true)
		// grab_focus on an unrooted widget (e.g. this call from New,
		// before Widget() is ever placed in a window) silently fails —
		// s.root's "realize" handler (wired in New) retries this once
		// the shell actually has a GtkRoot.
		s.composer.Focus()
	default:
		s.list.ListView().SetVisible(true)
		s.hint.SetVisible(false)
		s.searchEmpty.SetVisible(false)
	}
}
