package keymap

import (
	"log/slog"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// InstallNav attaches a PROPAGATION_CAPTURE key controller to widget
// (the notelist ListView) that resolves every key press against Table's
// ScopeList entries via Resolve and, for the subset Nav itself knows how
// to do (focus movement, section jumps, extend-selection, select-all-
// in-section — see navAction), runs the matching Nav method and calls
// onChanged so the caller can push Nav's resulting focus/selection back
// onto the real GtkSelectionModel and scroll position.
//
// This deliberately replaces GTK's own default arrow-key handling only
// for the keys Nav actually models: Nav has no notion of section
// boundaries or an anchor-and-extent selection the way the spec wants
// (GTK's own default Down/Up/Shift+Down/Shift+Up on a GtkListView +
// GtkMultiSelection does not know about section boundaries at all), so
// those need a real override; a plain click, by contrast, is left to
// the widget's own default mouse handling — see Nav.SyncFocus's doc
// comment for how the caller keeps Nav aligned with that instead of
// reimplementing it. Every other ScopeList entry (mark-done/delete-note,
// gated separately by InstallListGate; edit-inline/expand/collapse/
// move-note-up/move-note-down/copy/copy-as-list/merge/move-to, none of
// which Nav has any state for) falls through unhandled, exactly as if
// this controller were never installed.
func InstallNav(widget gtk.Widgetter, nav *Nav, onChanged func()) {
	ctrl := gtk.NewEventControllerKey()
	ctrl.SetPropagationPhase(gtk.PhaseCapture)
	ctrl.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) (handled bool) {
		// Resolve rebuilds its index from the package-level Table on
		// every call, which ApplyOverrides can mutate at runtime (a
		// config.toml [keys] override) — ApplyOverrides itself already
		// rejects anything that would make index's build fail (an
		// invalid accelerator string, a same-scope collision), so this
		// should never actually panic, but a raw GTK callback with no
		// wrapper of its own is exactly the shape every other such
		// callback in this codebase recovers, and a panic here would
		// otherwise kill the whole process on the very next keypress.
		defer func() {
			if r := recover(); r != nil {
				slog.Error("keymap: recovered panic in InstallNav's key handler", "panic", r)
				handled = false
			}
		}()
		e, ok := Resolve(ScopeList, keyval, state)
		if !ok || !navAction(nav, e.Action) {
			return false
		}
		if onChanged != nil {
			onChanged()
		}
		return true
	})
	gtk.BaseWidget(widget).AddController(ctrl)
}

// navAction runs the Nav method matching action and reports whether one
// exists. Split out from InstallNav so it is unit-testable with plain go
// test, with no GTK controller or display involved.
func navAction(nav *Nav, action string) bool {
	switch action {
	case "focus-next":
		nav.FocusNext()
	case "focus-previous":
		nav.FocusPrevious()
	case "jump-next-section":
		nav.JumpNextSection()
	case "jump-previous-section":
		nav.JumpPreviousSection()
	case "first-note":
		nav.FocusFirst()
	case "last-note":
		nav.FocusLast()
	case "extend-selection-down":
		nav.ExtendDown()
	case "extend-selection-up":
		nav.ExtendUp()
	case "select-all-section":
		nav.SelectAllInSection()
	default:
		return false
	}
	return true
}
