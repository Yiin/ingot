package panel

// SetNotice shows or clears a persistent, muted banner above the search
// bar — for a standing condition worth surfacing continuously, not a
// one-off toast: copper-l2z.30's "the global chord is off" degradation
// message is the first caller. An empty text hides the banner.
func (s *Shell) SetNotice(text string) {
	s.notice.SetText(text)
	s.notice.SetVisible(text != "")
}
