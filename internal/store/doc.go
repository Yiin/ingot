// Package store defines Ingot's note/project domain model — types, ids,
// sentinel errors, change events, and the Store interface — with no
// implementation and no I/O. Everything here uses the standard library
// only, so it is safe to import from anywhere, including internal/ui; the
// concrete implementation lives in internal/store/fsstore.
//
// A Store is not goroutine-safe. It must be called only from the
// goroutine that constructed it — in the app, the GTK main loop.
// Background work (file I/O, watching for external edits) hops back onto
// that goroutine through an injected Post func(func()), which the app
// sets to glib.IdleAdd; this package never imports GTK. Events fire
// synchronously on the caller's stack, which is what a gio.ListModel
// wants, so a Subscribe callback must not block.
package store
