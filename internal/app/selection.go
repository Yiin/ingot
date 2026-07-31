package app

import "github.com/Yiin/ingot/internal/ui/notelist"

// listSelection adapts notelist.List's own *Item-based Selected()/
// SelectItems() to menus.Selection's index-based contract — the sorted-
// order position space RowAt, ItemAtViewPosition, and every Handlers
// Row* query already share.
type listSelection struct {
	list *notelist.List
}

func (s listSelection) Selected() []int {
	items := s.list.Selected()
	out := make([]int, 0, len(items))
	for _, it := range items {
		if pos := s.list.ViewPositionOf(it); pos >= 0 {
			out = append(out, pos)
		}
	}
	return out
}

func (s listSelection) SetSelected(indices []int) {
	items := make([]*notelist.Item, 0, len(indices))
	for _, idx := range indices {
		if it := s.list.ItemAtViewPosition(idx); it != nil {
			items = append(items, it)
		}
	}
	s.list.SelectItems(items)
}
