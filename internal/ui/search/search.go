package search

import (
	"github.com/Yiin/ingot/internal/store/searchtext"
	"github.com/Yiin/ingot/internal/ui/notelist"
)

// Controller drives one notelist.List's live filter, note highlights,
// and focus-follows-search behavior.
type Controller struct {
	list  *notelist.List
	cache map[*notelist.Item]cacheEntry
}

// cacheEntry pairs a note's cached searchtext.Normalized form with the
// exact body it was computed from, so normalizedFor can tell a
// still-valid entry from a stale one with a plain string comparison
// instead of a body hash — mirrors internal/store/fsstore's own
// normCacheEntry.
type cacheEntry struct {
	body string
	norm searchtext.Normalized
}

// New returns a Controller driving list.
func New(list *notelist.List) *Controller {
	return &Controller{list: list, cache: make(map[*notelist.Item]cacheEntry)}
}

// Apply recomputes the panel's search state for query against list's
// current model (compute, this file's pure decision step) and pushes it
// into list: SetFilter's live visibility predicate, every real note's
// Ranges (consumed by RefreshHighlights and by every future bindRow),
// and — if the note that currently holds keyboard focus (list.Anchor())
// no longer shows — a new focus/selection on the first visible match.
//
// It returns the number of notes now visible: not the same as a
// searchtext Hit count, since a section whose own title matched shows
// every one of its notes whether or not any of them individually
// matched. That is deliberately the number both searchbar.SetMatchCount
// and panel.Shell.RefreshEmptyState want — the empty state must key off
// "is anything visible", which a body-match-only count would get wrong
// for a pure section-title search (RefreshEmptyState would show "No
// notes match" over a section that is, in fact, fully shown).
func (c *Controller) Apply(query string) int {
	model := c.list.Model()
	sections := model.Sections()
	items := model.Items()
	c.evictStale(items)

	r := c.compute(sections, items, c.list.Anchor(), query)

	if r.showAll {
		c.list.SetFilter(nil)
	} else {
		c.list.SetFilter(func(it *notelist.Item) bool {
			if !r.visible[it.SectionID] {
				return false
			}
			if it.IsPlaceholder() {
				// No notes to filter individually; the section's own
				// visibility (title match) already decided this.
				return true
			}
			return r.show[it]
		})
	}
	c.list.RefreshHighlights()

	if r.focus != nil {
		c.list.SelectItems([]*notelist.Item{r.focus})
		c.list.SetAnchor(r.focus)
		c.list.ScrollTo(r.focus)
	}

	return r.visibleCount
}

// computeResult is compute's pure output: everything Apply needs to push
// into the live GTK model, with no GTK type in sight.
type computeResult struct {
	// showAll is true for an empty query — every section and note shows,
	// mirroring the "no active filter" state
	// internal/store/searchtext.Filter itself documents.
	showAll bool
	// visible maps SectionID -> whether that section (header, rule, and
	// every note whose own show entry is true) should show at all.
	visible map[string]bool
	// show maps a real Item -> whether it should show. Never populated
	// for a placeholder; the filter predicate decides a placeholder's
	// visibility from its section's visible entry alone.
	show map[*notelist.Item]bool
	// visibleCount is the total number of real notes with show[it] true.
	visibleCount int
	// focus is the note the caller should move keyboard focus/selection
	// to, or nil if focus should be left exactly where it is (the
	// previously focused note is still visible, or there is nothing to
	// focus at all).
	focus *notelist.Item
}

// compute is Apply's pure decision step, split out so it is
// unit-testable without a live GTK display (constructing a real
// notelist.List needs one — see internal/ui/notelist's own integration-
// tag convention). It also mutates every it.Ranges in items directly: as
// the same pass already walks every note to test it against tokens, a
// second pass just to reset or populate Ranges would cost another O(n).
func (c *Controller) compute(sections []notelist.Section, items []*notelist.Item, focused *notelist.Item, query string) computeResult {
	tokens := searchtext.Tokens(query)
	if len(tokens) == 0 {
		for _, it := range items {
			it.Ranges = nil
		}
		return computeResult{showAll: true, visibleCount: len(items)}
	}

	bySection := make(map[string][]*notelist.Item, len(sections))
	for _, it := range items {
		bySection[it.SectionID] = append(bySection[it.SectionID], it)
	}

	visible := make(map[string]bool, len(sections))
	show := make(map[*notelist.Item]bool, len(items))
	visibleCount := 0
	var first *notelist.Item

	// bySection's per-section slices inherit items' base order, which is
	// exactly Item.seq order (Model.Append/InsertAt/Move all maintain
	// that invariant) — the same order the section's notes display in —
	// so the first shown item found while walking sections in their
	// declared order (also the display order, see notelist's
	// compareSection) is the first visible match without needing a
	// per-item notelist.Model.ViewPosition query, which is itself O(n)
	// and would make this whole pass O(n^2).
	for _, sec := range sections {
		titleMatched, _ := searchtext.Normalize(sec.Title).Match(tokens)
		anyShown := titleMatched
		for _, it := range bySection[sec.ID] {
			matched, ranges := c.normalizedFor(it).Match(tokens)
			it.Ranges = ranges
			shown := matched || titleMatched
			show[it] = shown
			if shown {
				anyShown = true
				visibleCount++
				if first == nil {
					first = it
				}
			}
		}
		visible[sec.ID] = anyShown
	}

	result := computeResult{visible: visible, show: show, visibleCount: visibleCount}
	stillShown := focused != nil && !focused.IsPlaceholder() && show[focused]
	if !stillShown && first != nil {
		result.focus = first
	}
	return result
}

// normalizedFor returns it's cached Normalized body, recomputing and
// re-caching it only when Body has changed since the cache was built —
// the common case, a search keystroke with no notes edited in between,
// costs one string comparison per note plus the substring scan itself,
// not a Markdown re-parse. Mirrors internal/store/fsstore's
// normalizedForLocked.
func (c *Controller) normalizedFor(it *notelist.Item) searchtext.Normalized {
	if e, ok := c.cache[it]; ok && e.body == it.Body {
		return e.norm
	}
	norm := searchtext.Normalize(it.Body)
	c.cache[it] = cacheEntry{body: it.Body, norm: norm}
	return norm
}

// evictStale drops cached entries for items no longer in the model, so
// the cache does not grow with every note ever seen in the panel's
// lifetime — mirrors internal/store/fsstore's evictSearchCacheLocked.
func (c *Controller) evictStale(items []*notelist.Item) {
	if len(c.cache) == 0 {
		return
	}
	live := make(map[*notelist.Item]bool, len(items))
	for _, it := range items {
		live[it] = true
	}
	for it := range c.cache {
		if !live[it] {
			delete(c.cache, it)
		}
	}
}
