package app

import (
	"context"
	"log/slog"
	"time"
)

// flushOnHideTimeout bounds hide()'s flush — a backstop against a stuck
// disk, not a normal-case latency budget: flushLocked short-circuits
// instantly when nothing is dirty, which is the common case.
const flushOnHideTimeout = 2 * time.Second

// show maps the panel, focused and ready to type into. Per the layer-
// shell doc, mapping hands keyboard focus to the surface automatically —
// no explicit focus-grab dance is needed beyond focusing the composer
// widget itself.
func (a *App) show() {
	if a.lsPanel == nil {
		return
	}
	a.lsPanel.Show()
	a.visible = true
	a.shell.SetFocused(true)
	a.shell.Composer().Focus()
}

// hide unmaps the panel. Per the layer-shell doc, focus returns to
// whatever toplevel was previously focused automatically. The flush
// runs synchronously, on the GTK thread hide() is always called from —
// never on a separate goroutine: Store.Flush's own doc says it "must be
// called only from the goroutine that constructed" the Store, since a
// conflict it detects fires ConflictResolved synchronously into the
// adapter, which then touches the GTK-only notelist.Model. It is
// usually instant anyway (flushLocked short-circuits when nothing is
// dirty); flushOnHideTimeout only backstops a stuck disk.
func (a *App) hide() {
	if a.lsPanel == nil {
		return
	}
	a.lsPanel.Hide()
	a.visible = false
	a.shell.SetFocused(false)

	if a.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), flushOnHideTimeout)
	defer cancel()
	if err := a.store.Flush(ctx); err != nil {
		slog.Warn("app: flush on hide", "err", err)
	}
}

// toggle is the "toggle" GAction's handler: a second `ingot` invocation
// (via gtkapp.ToggleRemote) or the compositor-bound global toggle
// binding both land here.
func (a *App) toggle() {
	if a.shell == nil {
		// startup hasn't finished yet — a toggle racing process launch.
		// Nothing to show or hide.
		return
	}
	if a.visible {
		a.hide()
	} else {
		a.show()
	}
}
