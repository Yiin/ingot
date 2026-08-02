package keymap

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// ShouldGateForList reports whether Space or BackSpace should be
// treated as a list action (mark-done, delete) rather than falling
// through to whatever widget would otherwise receive it, given whether
// the currently focused widget is a text-entry widget. textFocused
// comes from IsTextFocused in real use; a fake bool in tests.
//
// Space types a space in the composer and search field, and GTK list
// rows would otherwise swallow it; BackSpace must likewise never reach
// a focused text widget as a delete command. Both are only ever list
// actions while textFocused is false.
func ShouldGateForList(textFocused bool) bool {
	return !textFocused
}

// IsTextFocused reports whether w's root currently has a *gtk.Text
// focused — the check InstallListGate's PROPAGATION_CAPTURE key
// controller runs before gating Space or BackSpace to a list action.
// GtkEntry and GtkSearchEntry both delegate their own key handling to
// an internal GtkText, which is what Root.Focus returns while either is
// focused, so this covers the composer and the search field alike.
func IsTextFocused(w gtk.Widgetter) bool {
	root := gtk.BaseWidget(w).Root()
	if root == nil {
		return false
	}
	_, ok := root.Focus().(*gtk.Text)
	return ok
}

// InstallListGate attaches a PROPAGATION_CAPTURE key controller to
// widget (the notelist's ListView) that runs handlers[action] for every
// ScopeList accelerator in Table, but only while
// ShouldGateForList(IsTextFocused(widget)) is true. A key that resolves
// to no Table entry, or to an action with no handler, is reported
// unhandled, so GTK's normal bubble-phase delivery proceeds to whatever
// widget the user is actually aiming at — capture phase runs first, but
// returning false here does not stop that later delivery.
//
// This is where every bare-key list action has to live. An app-wide
// gtk.Application accelerator fires no matter what has focus, so binding
// Space or Return that way eats the character the user meant to type
// into the composer or the search field — which is why wireMenus clears
// those accelerators rather than relying on them.
//
// Resolving against Table rather than switching on keyvals is what keeps
// the shortcuts window honest: Table is what that window lists, so an
// entry gains its real behaviour by appearing in handlers under the same
// action name, and TestEveryListActionHasAHandler fails when one does
// not. Ctrl+Shift+Up/Down are the exception it permits, and its own
// comment says why.
func InstallListGate(widget gtk.Widgetter, handlers map[string]func()) {
	ctrl := gtk.NewEventControllerKey()
	ctrl.SetPropagationPhase(gtk.PhaseCapture)
	ctrl.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		if !ShouldGateForList(IsTextFocused(widget)) {
			return false
		}
		e, ok := Resolve(ScopeList, keyval, state)
		if !ok {
			return false
		}
		fn, ok := handlers[e.Action]
		if !ok {
			return false
		}
		fn()
		return true
	})
	gtk.BaseWidget(widget).AddController(ctrl)
}
