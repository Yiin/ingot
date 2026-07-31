// Package layershell wraps gtk4-layer-shell (no Go binding exists for
// GTK4) with a hand-written cgo shim, so the panel can dock to a screen
// edge as a layer-shell surface.
//
// Along with internal/ui, this is one of only two packages allowed to
// import cgo. Keeping cgo out of every other package is what lets
// `go test ./internal/store/...` and friends run without a GTK runtime,
// in CI and otherwise.
package layershell
