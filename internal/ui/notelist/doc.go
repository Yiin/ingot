// Package notelist is the panel's scrolling body: a GtkListView over a
// gioutil-boxed GListModel, grouped into section headers, with
// multi-selection, an overlay scrollbar, and an insert animation for newly
// captured notes.
//
// # The Item/Section boundary
//
// internal/store (the on-disk domain model) does not exist yet as of this
// package. notelist therefore defines its own view-model, Item and
// Section, decoupled from any future store.Note. internal/store must never
// import notelist — only internal/ui packages may use cgo, so the
// dependency runs one way: a future adapter (in the app-wiring layer, not
// here) converts store.Note into notelist.Item.
//
// # Threading
//
// Every exported method reads or writes live GTK/GLib state and must run
// on the GTK main thread. There is no internal locking.
package notelist
