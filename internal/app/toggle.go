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

// show maps the panel, focused and ready to type into. Present rather
// than SetVisible(true): the panel is an ordinary toplevel now, so a
// toggle fired while it is already up behind another window should raise
// and focus it, which only Present does. The .unfocused styling follows
// the window's own is-active property (see startup), so show does not set
// it here.
func (a *App) show() {
	if a.win == nil {
		return
	}
	a.win.Present()
	a.visible = true
	a.shell.Composer().Focus()
}

// hide unmaps the panel. The compositor hands focus back to whatever
// toplevel was previously focused. The size is captured first, while the
// window is still mapped and can report one. The flush
// runs synchronously, on the GTK thread hide() is always called from —
// never on a separate goroutine: Store.Flush's own doc says it "must be
// called only from the goroutine that constructed" the Store, since a
// conflict it detects fires ConflictResolved synchronously into the
// adapter, which then touches the GTK-only notelist.Model. It is
// usually instant anyway (flushLocked short-circuits when nothing is
// dirty); flushOnHideTimeout only backstops a stuck disk.
func (a *App) hide() {
	if a.win == nil {
		return
	}
	a.savePanelSize()
	a.win.SetVisible(false)
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
