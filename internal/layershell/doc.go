// Package layershell wraps gtk4-layer-shell (no Go binding exists for
// GTK4) with a hand-written cgo shim (shim.go), and owns the panel's
// surface lifecycle on top of it (panel.go): docking to the right
// screen edge, map/unmap show and hide, keyboard focus policy, and
// output (monitor) selection. It never touches widgets or the window's
// content — that is internal/ui's job.
//
// Along with internal/ui, this is one of only two packages allowed to
// import cgo. Keeping cgo out of every other package is what lets
// `go test ./internal/store/...` and friends run without a GTK runtime,
// in CI and otherwise.
package layershell
