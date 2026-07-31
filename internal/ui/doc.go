// Package ui builds the GTK panel: the note list, composer, search bar,
// menus, and every other on-screen widget.
//
// Along with internal/layershell, this is one of only two packages allowed
// to import cgo (directly, or transitively via gotk4). Keeping cgo out of
// every other package is what lets `go test ./internal/store/...` and
// friends run without a GTK runtime, in CI and otherwise.
package ui
