package panel

import "github.com/Yiin/ingot/internal/ui/notelist"

// NotifyEmptySelection is the panel's response to a capture chord firing
// over an empty text selection: create no note, show the dark HUD
// reading "Nothing selected". Deciding that the selection was empty is
// the caller's job (copper-l2z.30) — Shell has no selection dependency
// of its own — so this only ever renders the outcome. Notifier.Captured
// hops onto the GTK thread itself (see internal/ui/toast), so this is
// safe to call from any goroutine.
func (s *Shell) NotifyEmptySelection() {
	s.notifier.Captured("Nothing selected")
}

// NotifyDuplicate is the panel's response to a capture chord duplicating
// the newest note: create no duplicate, flash the existing row's ring
// twice, and show the dark HUD reading "Already captured". Deciding that
// the capture was a duplicate, and which Item it duplicates, is the
// caller's job (copper-l2z.30) — this only renders the outcome.
func (s *Shell) NotifyDuplicate(it *notelist.Item) {
	s.list.FlashDuplicate(it)
	s.notifier.Captured("Already captured")
}
