// Package menus builds Ingot's context menu, Move to submenu, and
// overflow menu, and registers every app-level action and accelerator
// they (and their keyboard shortcuts) run against.
//
// # Actions live on the application, not a window
//
// Register installs each gio.SimpleAction on the *gtk.Application (the
// "app." prefix) so accelerators fire whether or not a menu is open, and
// calls SetAccelsForAction for every action that has one. Radio ticks and
// checkmarks on "project", "keep-on-top", and "clear-done" come from
// those actions' own state — no menu item ever carries a hand-written
// check attribute.
//
// GTK normalises modifier order for display (you write <Control><Shift>c,
// AccelsForAction returns <Shift><Control>c), so nothing in this package
// or its callers ever string-compares an accel.
//
// # move-to's target encoding
//
// "move-to" is parameterised (VariantType("s")) but deliberately not
// stateful — it is a one-shot command, not a persisted selection. Its
// string parameter is one of:
//
//	"section:<id>"  move the selection into an existing section
//	"project:<id>"  move the selection into a project's first section
//
// # Rendering an item insensitive without a state trick
//
// GTK's menu tracker (verified against GTK 4.22's own source) only
// renders an item insensitive when it is bound to an action name that
// does not exist, or that exists but is disabled — an item with no
// action attribute at all renders sensitive, just inert. So the Move to
// submenu's current-section item, which needs to look ticked but not be
// clickable, is bound to moveToCurrentSectionAction, a name Register
// never installs, rather than left unbound. The overflow menu's "Window"
// label uses a different, cleaner mechanism: it is a genuine GMenu
// section heading (AppendSection("Window", ...)), which GTK renders as
// non-interactive text by construction — no insensitivity trick needed.
//
// # New Section's custom child must be re-added after every rebuild
//
// "New Section..." is not a GAction either: it needs a text prompt, which
// does not fit the fire-and-forget action model. Its menu item carries
// the "custom" attribute NewSectionCustomID; a caller embeds the actual
// entry widget via (*gtk.PopoverMenu).AddChild. GTK discards a popover's
// custom children every time its menu model is replaced, and
// ContextMenuController replaces the model on every right-click, so the
// caller must re-add the entry each time — register it through
// ContextMenuController.SetOnRebuilt rather than adding it once.
//
// # Clear Done's two-activation confirm is a boolean-state action
//
// Clear Done is destructive and confirms inline, never with a modal
// dialog. Despite the spec naming only "project" and "keep-on-top"
// stateful, "clear-done" is a boolean-state gio.SimpleAction too: a
// stateless action's menu item always closes the popover on activation
// (GTK's NORMAL item role), which would make a two-click confirm land on
// two different popover openings instead of one. A boolean-state item
// gets GTK's CHECK role, which keeps the popover open across both
// clicks. The first activation (false -> true) arms the confirmation and
// shows a checkmark; the second (true -> false) is read as the confirm
// and calls Handlers.ClearDone. AttachOverflow wires the popover's
// "closed" signal to Actions.ResetClearDone, so an abandoned first click
// does not stay armed into the next time the menu opens.
package menus
