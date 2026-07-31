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
// widget (the notelist's ListView) that calls onMarkDone for Space and
// onDelete for BackSpace, but only while ShouldGateForList(IsTextFocused
// (widget)) is true. Otherwise it reports the key unhandled, so GTK's
// normal bubble-phase delivery proceeds to whatever text widget the
// user is actually typing into — capture phase runs first, but
// returning false here does not stop that later delivery.
func InstallListGate(widget gtk.Widgetter, onMarkDone, onDelete func()) {
	ctrl := gtk.NewEventControllerKey()
	ctrl.SetPropagationPhase(gtk.PhaseCapture)
	ctrl.ConnectKeyPressed(func(keyval, _ uint, _ gdk.ModifierType) bool {
		if !ShouldGateForList(IsTextFocused(widget)) {
			return false
		}
		switch keyval {
		case gdk.KEY_space:
			onMarkDone()
			return true
		case gdk.KEY_BackSpace:
			onDelete()
			return true
		}
		return false
	})
	gtk.BaseWidget(widget).AddController(ctrl)
}
