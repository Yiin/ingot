package app

import (
	"log/slog"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/keymap"
)

// escapeTarget adapts the live panel to keymap.EscapeTarget. It is a
// distinct type rather than methods on *App because several of the
// cascade's names (ClearSelection, FocusComposer, HidePanel) would
// otherwise collide with, or shadow, App's own menu-action methods.
type escapeTarget struct{ a *App }

// PopoverOpen reports whether either menu popover is on screen: the
// right-click context menu, or the search bar's overflow menu. Both are
// ordinary GtkPopovers, so visibility is the whole question.
func (t escapeTarget) PopoverOpen() bool {
	return t.contextPopoverOpen() || t.overflowPopoverOpen()
}

func (t escapeTarget) ClosePopover() {
	if t.contextPopoverOpen() {
		t.a.ctxMenu.Popover().Popdown()
		return
	}
	if t.overflowPopoverOpen() {
		t.a.shell.SearchBar().OverflowButton().Popover().Popdown()
	}
}

func (t escapeTarget) contextPopoverOpen() bool {
	if t.a.ctxMenu == nil {
		return false
	}
	p := t.a.ctxMenu.Popover()
	return p != nil && p.IsVisible()
}

func (t escapeTarget) overflowPopoverOpen() bool {
	btn := t.a.shell.SearchBar().OverflowButton()
	if btn == nil {
		return false
	}
	p := btn.Popover()
	return p != nil && p.IsVisible()
}

func (t escapeTarget) EditingInline() bool { return t.a.shell.List().EditingInline() }
func (t escapeTarget) CancelInlineEdit()   { t.a.shell.List().CancelInlineEdit() }

func (t escapeTarget) SearchHasText() bool { return t.a.shell.SearchBar().Text() != "" }

// ClearSearchText empties the entry, which fires its own changed handler
// and so re-runs the filter — no separate refresh needed here.
func (t escapeTarget) ClearSearchText() { t.a.shell.SearchBar().Clear() }

func (t escapeTarget) HasSelection() bool { return len(t.a.shell.List().Selected()) > 0 }
func (t escapeTarget) ClearSelection()    { t.a.shell.List().ClearSelection() }

func (t escapeTarget) ComposerFocused() bool { return t.a.shell.Composer().Focused() }
func (t escapeTarget) FocusComposer()        { t.a.shell.Composer().Focus() }

func (t escapeTarget) HidePanel() { t.a.hide() }

// wireEscape installs the Escape cascade on the panel window.
//
// keymap.HandleEscape and its EscapeTarget contract existed, fully unit
// tested, with no caller anywhere in the running app — Escape did
// nothing at all, so once focus was in the composer the only way out was
// Ctrl+W (copper-l2z.85).
//
// The capture phase is load-bearing, for the same reason composer's own
// Return handler needs it: the focused GtkText or GtkTextView consumes
// Escape in the bubble phase before a window-level handler would ever
// run. Capturing at the window means the cascade sees Escape first, from
// wherever focus happens to be.
//
// This is deliberately not a keymap.Table entry: Table's accelerators go
// through GtkApplication, which would fire Escape app-wide even while a
// popover has a grab, and popover dismissal is the cascade's own first
// step.
func (a *App) wireEscape() {
	target := escapeTarget{a: a}

	ctrl := gtk.NewEventControllerKey()
	ctrl.SetPropagationPhase(gtk.PhaseCapture)
	ctrl.ConnectKeyPressed(func(keyval, _ uint, _ gdk.ModifierType) bool {
		if keyval != gdk.KEY_Escape {
			return false
		}
		defer guard("escape")()
		slog.Debug("app: escape", "step", keymap.HandleEscape(target))
		return true
	})
	a.win.AddController(ctrl)
}
