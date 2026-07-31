// Package wl probes which Wayland compositor globals are present via a
// single wl_registry roundtrip.
//
// No maintained Go binding for the Wayland wire protocol exists, and the
// four globals this package cares about — ext_data_control_manager_v1,
// zwlr_data_control_manager_v1, zwlr_layer_shell_v1, and
// zwp_virtual_keyboard_manager_v1 — are all advertised by the base
// wl_registry, so this package speaks just enough of the core protocol by
// hand: connect, wl_display.get_registry, wl_display.sync, and read
// wl_registry.global events until the sync callback fires. That is a
// handful of fixed-shape messages, not a generated binding, and it keeps
// this package cgo-free like the rest of internal/store and internal/input.
package wl
