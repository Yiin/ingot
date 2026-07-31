// Package setup installs the udev rule that lets Ingot read keyboard
// input without running as root or joining the input group, and backs
// the "ingot setup" and "ingot doctor" subcommands with each first-run
// health check.
//
// Nothing here touches cgo: only internal/ui and internal/layershell may,
// so checks that would naturally ask GTK a question (e.g. whether
// gtk4-layer-shell is supported) instead ask internal/wl for the
// equivalent compositor capability.
package setup
