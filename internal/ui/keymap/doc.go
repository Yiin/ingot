// Package keymap is the single source of truth for every Ingot key
// binding, mouse gesture, and the global double-Shift capture chord.
//
// Table drives two things from the exact same data: the real bindings
// (an app-wide GTK accelerator for a Scope of ScopeApp, a custom key
// controller checked against Scope of ScopeList — see the package's
// gating and navigation halves — and internal/hotkey for ScopeGlobal)
// and the generated GtkShortcutsWindow (see shortcuts.go). The sheet is
// never hand-maintained separately from the bindings it documents.
//
// Nav (nav.go) is the notelist's keyboard/mouse navigation and
// selection engine: an anchor-and-extent selection over a fixed display
// order, with no GTK dependency, so it is testable with plain go test.
//
// HandleEscape (escape.go) runs the documented six-step Escape cascade:
// close a popover or menu, cancel an inline edit, clear the search
// text, clear the selection, return focus to the composer, hide the
// panel — stopping at the first step that applies.
//
// ShouldGateForList (gating.go) is the policy behind "Space must be
// gated": Space and BackSpace type into a focused text field exactly as
// GTK's own widgets already do, and only become list actions
// (mark-done, delete) when the list itself holds keyboard focus. Wire
// it from a key controller in PROPAGATION_CAPTURE, ahead of the
// focused widget's own handling.
package keymap
