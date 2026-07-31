package motion

// ShouldScroll reports whether a row landing at pos (0-based, in display
// order) should trigger the ScrollToInsertedDuration scroll animation,
// given [firstVisible, lastVisible] as the range of positions currently
// on screen (both inclusive). Per the spec: scroll only if the row lands
// outside the viewport — a row that inserts already visible must trigger
// zero scroll animations, and one that inserts off-screen must trigger
// exactly one.
//
// This is a plain decision function, not itself a scroll call: the
// caller (the end-to-end wiring that knows both the freshly inserted
// row's position and the list's current visible range) calls
// notelist.List.ScrollToAndSelect itself, at most once, only when this
// returns true.
func ShouldScroll(pos, firstVisible, lastVisible int) bool {
	return pos < firstVisible || pos > lastVisible
}
