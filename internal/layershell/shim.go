package layershell

/*
#cgo pkg-config: gtk4-layer-shell-0
#include <gtk4-layer-shell.h>
#include <stdlib.h>
*/
import "C"

import (
	"runtime"
	"unsafe"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Layer mirrors GtkLayerShellLayer: which stacking layer a surface
// appears on relative to normal toplevels.
type Layer int

const (
	LayerBackground Layer = C.GTK_LAYER_SHELL_LAYER_BACKGROUND
	LayerBottom     Layer = C.GTK_LAYER_SHELL_LAYER_BOTTOM
	LayerTop        Layer = C.GTK_LAYER_SHELL_LAYER_TOP
	LayerOverlay    Layer = C.GTK_LAYER_SHELL_LAYER_OVERLAY
)

// Edge mirrors GtkLayerShellEdge: one of the four screen edges a surface
// can be anchored to or margined from.
type Edge int

const (
	EdgeLeft   Edge = C.GTK_LAYER_SHELL_EDGE_LEFT
	EdgeRight  Edge = C.GTK_LAYER_SHELL_EDGE_RIGHT
	EdgeTop    Edge = C.GTK_LAYER_SHELL_EDGE_TOP
	EdgeBottom Edge = C.GTK_LAYER_SHELL_EDGE_BOTTOM
)

// KeyboardMode mirrors GtkLayerShellKeyboardMode.
type KeyboardMode int

const (
	// KeyboardModeNone means the surface never receives keyboard events.
	KeyboardModeNone KeyboardMode = C.GTK_LAYER_SHELL_KEYBOARD_MODE_NONE
	// KeyboardModeExclusive means the surface grabs keyboard focus
	// permanently while mapped and refuses to yield it. Never used by
	// Ingot: see Panel's doc comment for why ON_DEMAND is the right
	// choice even though the two are indistinguishable on Hyprland.
	KeyboardModeExclusive KeyboardMode = C.GTK_LAYER_SHELL_KEYBOARD_MODE_EXCLUSIVE
	// KeyboardModeOnDemand means the surface can be focused and
	// unfocused in a compositor-defined way. This is what Panel uses.
	KeyboardModeOnDemand KeyboardMode = C.GTK_LAYER_SHELL_KEYBOARD_MODE_ON_DEMAND
)

// IsSupported reports whether the current platform is Wayland and the
// compositor advertises the zwlr_layer_shell_v1 protocol. It may block
// for a Wayland roundtrip the first time it is called, and must not be
// called before gtk.Init.
func IsSupported() bool {
	return C.gtk_layer_is_supported() != C.FALSE
}

// nativeWindow returns the underlying *C.GtkWindow for w.
//
// gotk4's *gtk.Widget declares its own Native() method returning
// *gtk.NativeSurface (the nearest GtkNative ancestor, i.e. GTK's own
// higher-level concept), which shadows the raw-pointer Native() uintptr
// promoted from the embedded glib.Object. So w.Native() does not give a
// C pointer here; the idiom gotk4's own generated code uses instead is
// coreglib.InternObject(w).Native(), which this mirrors. The uintptr ->
// unsafe.Pointer conversion is why `go vet` needs -unsafeptr=false (see
// Makefile and .golangci.yml).
func nativeWindow(w *gtk.Window) *C.GtkWindow {
	return (*C.GtkWindow)(unsafe.Pointer(coreglib.InternObject(w).Native()))
}

func cBool(b bool) C.gboolean {
	if b {
		return C.TRUE
	}
	return C.FALSE
}

// InitForWindow turns w into a layer-shell surface once it is mapped.
// Must be called before w is realized, and before any of the setters
// below.
func InitForWindow(w *gtk.Window) {
	C.gtk_layer_init_for_window(nativeWindow(w))
	runtime.KeepAlive(w)
}

// SetNamespace sets the surface's namespace, the identifier compositor
// tooling and config (e.g. Hyprland windowrulev2) match against.
func SetNamespace(w *gtk.Window, namespace string) {
	cNamespace := C.CString(namespace)
	defer C.free(unsafe.Pointer(cNamespace))
	C.gtk_layer_set_namespace(nativeWindow(w), cNamespace)
	runtime.KeepAlive(w)
}

// SetLayer sets which stacking layer the surface appears on.
func SetLayer(w *gtk.Window, layer Layer) {
	C.gtk_layer_set_layer(nativeWindow(w), C.GtkLayerShellLayer(layer))
	runtime.KeepAlive(w)
}

// SetAnchor anchors (or unanchors) the surface to edge. An axis with
// neither edge anchored is centred on that axis by the wlr-layer-shell
// convention.
func SetAnchor(w *gtk.Window, edge Edge, anchor bool) {
	C.gtk_layer_set_anchor(nativeWindow(w), C.GtkLayerShellEdge(edge), cBool(anchor))
	runtime.KeepAlive(w)
}

// SetMargin sets the surface's distance from edge, effective only when
// edge is anchored.
func SetMargin(w *gtk.Window, edge Edge, marginSize int) {
	C.gtk_layer_set_margin(nativeWindow(w), C.GtkLayerShellEdge(edge), C.int(marginSize))
	runtime.KeepAlive(w)
}

// SetExclusiveZone requests that the compositor not place other
// surfaces within the given distance of the anchored edge. Zero means
// the surface reserves no space and other windows are not pushed.
func SetExclusiveZone(w *gtk.Window, zone int) {
	C.gtk_layer_set_exclusive_zone(nativeWindow(w), C.int(zone))
	runtime.KeepAlive(w)
}

// SetKeyboardMode sets if/when the surface receives keyboard events.
func SetKeyboardMode(w *gtk.Window, mode KeyboardMode) {
	C.gtk_layer_set_keyboard_mode(nativeWindow(w), C.GtkLayerShellKeyboardMode(mode))
	runtime.KeepAlive(w)
}

// SetMonitor pins the surface to monitor, or lets the compositor choose
// the output when monitor is nil (the default).
func SetMonitor(w *gtk.Window, monitor *gdk.Monitor) {
	var cMonitor *C.GdkMonitor
	if monitor != nil {
		cMonitor = (*C.GdkMonitor)(unsafe.Pointer(coreglib.InternObject(monitor).Native()))
	}
	C.gtk_layer_set_monitor(nativeWindow(w), cMonitor)
	runtime.KeepAlive(w)
	runtime.KeepAlive(monitor)
}
