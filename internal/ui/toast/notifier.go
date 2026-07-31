package toast

// Notifier is the app-wide toast surface every capture/action flow
// reports through.
//
// Captured shows the dark global HUD, for every outcome of the global
// capture chord: a successful "Captured: <text>" (the captured text is
// shown so a stale-PRIMARY capture is visible to the user), "Nothing
// selected", and "Already captured". All three fire from a global
// gesture that can land while the panel is hidden or unfocused, so all
// three go through the HUD, never the in-panel toast.
//
// Message shows the light in-panel toast, for feedback on an action that
// requires the panel to already be visible — "Copied as List" is the
// spec's only measured instance.
//
// Both methods are safe to call from any goroutine — the capture flow
// that calls Captured runs on the evdev reader goroutine, not the GTK
// main thread. See Toaster's doc comment for how the concrete
// implementation hops onto the GTK thread.
type Notifier interface {
	Captured(text string)
	Message(text string)
}

// Nop is a Notifier that discards every call, for tests and any caller
// (e.g. a doctor-style probe) that needs the interface without a live
// UI.
type Nop struct{}

func (Nop) Captured(string) {}
func (Nop) Message(string)  {}

var _ Notifier = Nop{}
