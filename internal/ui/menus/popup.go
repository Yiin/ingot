package menus

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// RowLocator translates a right-click's pixel position — relative to the
// widget the context-menu gesture is attached to — into a row index and
// that row's own widget. The eventual list assembly (the notelist/panel
// children) implements this; menus stays agnostic of how rows are laid
// out or recycled.
type RowLocator interface {
	RowAt(x, y float64) (row gtk.Widgetter, index int, ok bool)
}

// Selection abstracts the current multi-selection so
// ContextMenuController can apply the right-click targeting rule without
// owning selection state itself — the same selection the rest of the
// panel reads and renders from.
type Selection interface {
	Selected() []int
	SetSelected(indices []int)
}

// ResolveClickSelection implements the right-click targeting rule: a
// right-click on a row already inside the current selection leaves the
// whole selection untouched, so Merge Notes and Move to can act on it. A
// right-click on any other row replaces the selection with just that row.
func ResolveClickSelection(clicked int, selected []int) []int {
	for _, idx := range selected {
		if idx == clicked {
			return selected
		}
	}
	return []int{clicked}
}

// ContextMenuController owns the one shared GtkPopoverMenu the note
// context menu uses, per the spec: a single popover parented to the
// window, repositioned and rebuilt for every right-click rather than
// recreated.
type ContextMenuController struct {
	popover   *gtk.PopoverMenu
	window    *gtk.Window
	actions   *Actions
	handlers  Handlers
	locator   RowLocator
	selection Selection
	onRebuilt func(popover *gtk.PopoverMenu)

	target int
}

// NewContextMenuController creates the shared popover, parents it to
// window, and attaches a secondary-button GestureClick to attachTo (the
// widget hosting the note rows).
func NewContextMenuController(
	window *gtk.Window, attachTo gtk.Widgetter, actions *Actions, h Handlers,
	locator RowLocator, selection Selection,
) *ContextMenuController {
	popover := gtk.NewPopoverMenuFromModel(nil)
	popover.SetParent(window)
	popover.SetHasArrow(false)

	c := &ContextMenuController{
		popover:   popover,
		window:    window,
		actions:   actions,
		handlers:  h,
		locator:   locator,
		selection: selection,
		target:    -1,
	}

	gesture := gtk.NewGestureClick()
	gesture.SetButton(gdk.BUTTON_SECONDARY)
	gesture.ConnectPressed(func(nPress int, x, y float64) {
		c.onPressed(gesture, attachTo, x, y)
	})
	gtk.BaseWidget(attachTo).AddController(gesture)

	return c
}

// Popover returns the shared popover, so the caller can inspect it
// directly if needed. To embed the "New Section..." entry widget, use
// SetOnRebuilt instead: SetMenuModel replaces every custom child GTK
// attached via (*gtk.PopoverMenu).AddChild, so the entry has to be
// re-added after every rebuild, not just once.
func (c *ContextMenuController) Popover() *gtk.PopoverMenu {
	return c.popover
}

// SetOnRebuilt registers a callback that runs immediately after every
// SetMenuModel call this controller makes (i.e. after every right-click).
// Use it to re-attach any (*gtk.PopoverMenu).AddChild custom widgets —
// most notably the Move to submenu's "New Section..." entry under
// NewSectionCustomID — since GTK discards a popover's custom children
// each time its menu model is replaced.
func (c *ContextMenuController) SetOnRebuilt(fn func(popover *gtk.PopoverMenu)) {
	c.onRebuilt = fn
}

// Target returns the row index the currently open (or last opened)
// context menu targets, or -1 before any right-click.
func (c *ContextMenuController) Target() int {
	return c.target
}

// onPressed resolves the click to a row, applies the right-click
// targeting rule, syncs Actions.Expand/Merge's enablement (a GMenu
// item's sensitivity always follows its bound action, never the model),
// builds a fresh context menu, points the shared popover at the clicked
// row, and shows it. It claims the click sequence so the underlying list
// widget's own click handling (e.g. left-click row selection) does not
// also fire for this right-click.
func (c *ContextMenuController) onPressed(gesture *gtk.GestureClick, attachTo gtk.Widgetter, x, y float64) {
	_, index, ok := c.locator.RowAt(x, y)
	if !ok {
		return
	}
	gesture.SetState(gtk.EventSequenceClaimed)
	c.target = index

	newSelection := ResolveClickSelection(index, c.selection.Selected())
	c.selection.SetSelected(newSelection)

	c.actions.Expand.SetEnabled(c.handlers.RowIsTruncated(index))
	c.actions.RefreshMergeEnablement(len(newSelection))

	c.popover.SetMenuModel(BuildContext(ContextInfo{
		Done:             c.handlers.RowIsDone(index),
		Expanded:         c.handlers.RowIsExpanded(index),
		CurrentProjectID: c.handlers.CurrentProjectID(),
		CurrentSectionID: c.handlers.CurrentSectionID(),
		Sections:         c.handlers.Sections(),
		OtherProjects:    otherProjects(c.handlers.Projects(), c.handlers.CurrentProjectID()),
	}))
	if c.onRebuilt != nil {
		c.onRebuilt(c.popover)
	}

	wx, wy, ok := gtk.BaseWidget(attachTo).TranslateCoordinates(c.window, x, y)
	if !ok {
		return
	}
	rect := gdk.NewRectangle(int(wx), int(wy), 1, 1)
	c.popover.SetPointingTo(&rect)
	c.popover.Popup()
}

// otherProjects returns projects with currentID removed, preserving
// order.
func otherProjects(projects []Project, currentID string) []Project {
	out := make([]Project, 0, len(projects))
	for _, p := range projects {
		if p.ID == currentID {
			continue
		}
		out = append(out, p)
	}
	return out
}

// AttachOverflow wires the overflow menu onto button via
// SetCreatePopupFunc, which GTK calls immediately before every popup —
// from a mouse click or a keybinding alike — unlike MenuButton's
// "activate" signal, which only fires for the latter. Each call builds a
// fresh popover from the current project list and hands it to
// SetPopover, and wires that same popover's "closed" signal to disarm
// Clear Done, so the disarm handler never goes stale the way it would if
// a later refresh replaced the popover out from under it.
func AttachOverflow(button *gtk.MenuButton, h Handlers, actions *Actions) {
	button.SetCreatePopupFunc(func(mb *gtk.MenuButton) {
		popover := gtk.NewPopoverMenuFromModel(BuildOverflow(h.Projects()))
		popover.ConnectClosed(func() {
			actions.ResetClearDone()
		})
		mb.SetPopover(&popover.Popover)
	})
}
