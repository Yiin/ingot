package keymap

import (
	"sort"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Scope limits where a binding is active.
type Scope uint8

const (
	// ScopeGlobal is active system-wide, independent of window focus —
	// read from evdev by internal/hotkey, never through GTK at all.
	ScopeGlobal Scope = iota
	// ScopeApp is active anywhere in the app, installed as a real
	// gtk.Application accelerator.
	ScopeApp
	// ScopeList is active only while the note list holds keyboard
	// focus. It is never installed as an app-wide accelerator — doing
	// so would fire it even while a text field is focused — a
	// dedicated key controller on the list widget checks Entry.Accels
	// for ScopeList entries instead.
	ScopeList
)

// Group is a GtkShortcutsWindow category. Groups gives the fixed
// display order the window's sections follow.
type Group string

const (
	GroupCapture    Group = "Capture"
	GroupNavigation Group = "Navigation"
	GroupSelection  Group = "Selection"
	GroupEditing    Group = "Editing"
	GroupClipboard  Group = "Clipboard"
	GroupProjects   Group = "Projects"
	GroupWindow     Group = "Window"
)

// Groups is the fixed display order BuildSections groups Table's
// entries into.
var Groups = []Group{
	GroupCapture,
	GroupNavigation,
	GroupSelection,
	GroupEditing,
	GroupClipboard,
	GroupProjects,
	GroupWindow,
}

// Entry is one row of the keymap: a stable Action id, the GTK
// accelerator strings (gtk.AcceleratorParse syntax, e.g.
// "<Control><Shift>c") that trigger it, and where it belongs in the
// generated GtkShortcutsWindow.
//
// Accels is empty for a binding GTK cannot express as an accelerator —
// the global double-Shift chord, or a mouse gesture — in which case
// Display carries the human-readable description the shortcuts window
// shows in place of an accelerator chip.
type Entry struct {
	// Action is a stable identifier, kebab-case, unique across Table.
	Action string
	// Title is the shortcuts-window row label.
	Title string
	// Accels are gtk.AcceleratorParse-syntax strings; every accelerator
	// in Table maps to exactly one Entry (see TestAccelsAreUnique).
	Accels []string
	// Display is the shortcuts-window subtitle for an Entry whose
	// Accels is empty.
	Display string
	Scope   Scope
	Group   Group
}

// Table is the single source of truth for every Ingot binding — see
// the package doc.
var Table = []Entry{
	{
		Action:  "global-capture",
		Title:   "Capture the current text selection",
		Display: "Shift Shift — double-tap within 400ms, no other key between",
		Scope:   ScopeGlobal,
		Group:   GroupCapture,
	},
	{
		Action: "toggle-panel",
		Title:  "Toggle panel",
		Accels: []string{"<Super><Shift>c"},
		Scope:  ScopeGlobal,
		Group:  GroupWindow,
	},

	{Action: "focus-next", Title: "Focus next note", Accels: []string{"Down"}, Scope: ScopeList, Group: GroupNavigation},
	{Action: "focus-previous", Title: "Focus previous note", Accels: []string{"Up"}, Scope: ScopeList, Group: GroupNavigation},
	{Action: "jump-next-section", Title: "Jump to next section", Accels: []string{"<Control>Down"}, Scope: ScopeList, Group: GroupNavigation},
	{Action: "jump-previous-section", Title: "Jump to previous section", Accels: []string{"<Control>Up"}, Scope: ScopeList, Group: GroupNavigation},
	{Action: "first-note", Title: "First note", Accels: []string{"Home"}, Scope: ScopeList, Group: GroupNavigation},
	{Action: "last-note", Title: "Last note", Accels: []string{"End"}, Scope: ScopeList, Group: GroupNavigation},
	{Action: "focus-search", Title: "Focus search", Accels: []string{"<Control>f"}, Scope: ScopeApp, Group: GroupNavigation},
	{Action: "focus-composer", Title: "Focus composer", Accels: []string{"<Control>n"}, Scope: ScopeApp, Group: GroupNavigation},

	{Action: "extend-selection-down", Title: "Extend selection down", Accels: []string{"<Shift>Down"}, Scope: ScopeList, Group: GroupSelection},
	{Action: "extend-selection-up", Title: "Extend selection up", Accels: []string{"<Shift>Up"}, Scope: ScopeList, Group: GroupSelection},
	{
		Action:  "toggle-selection-click",
		Title:   "Toggle one note in the selection",
		Display: "Ctrl+click a note",
		Scope:   ScopeList,
		Group:   GroupSelection,
	},
	{
		Action:  "range-select-click",
		Title:   "Range select",
		Display: "Shift+click a note",
		Scope:   ScopeList,
		Group:   GroupSelection,
	},
	{Action: "select-all-section", Title: "Select all in section", Accels: []string{"<Control>a"}, Scope: ScopeList, Group: GroupSelection},

	{Action: "move-note-up", Title: "Move note up", Accels: []string{"<Control><Shift>Up"}, Scope: ScopeList, Group: GroupEditing},
	{Action: "move-note-down", Title: "Move note down", Accels: []string{"<Control><Shift>Down"}, Scope: ScopeList, Group: GroupEditing},
	{Action: "mark-done", Title: "Mark done / undone", Accels: []string{"space"}, Scope: ScopeList, Group: GroupEditing},
	{Action: "edit-inline", Title: "Edit inline", Accels: []string{"Return"}, Scope: ScopeList, Group: GroupEditing},
	{Action: "edit-new-window", Title: "Edit in new window", Accels: []string{"<Control>Return"}, Scope: ScopeList, Group: GroupEditing},
	{Action: "expand", Title: "Expand note", Accels: []string{"Right"}, Scope: ScopeList, Group: GroupEditing},
	{Action: "collapse", Title: "Collapse note", Accels: []string{"Left"}, Scope: ScopeList, Group: GroupEditing},
	{Action: "toggle-expand", Title: "Expand / collapse note", Accels: []string{"<Alt>Return"}, Scope: ScopeList, Group: GroupEditing},
	{Action: "merge", Title: "Merge notes (2+ selected)", Accels: []string{"<Control><Shift>m"}, Scope: ScopeList, Group: GroupEditing},
	{Action: "delete-note", Title: "Delete note", Accels: []string{"Delete", "BackSpace"}, Scope: ScopeList, Group: GroupEditing},
	{Action: "move-to", Title: "Move to...", Accels: []string{"<Control><Shift>v"}, Scope: ScopeList, Group: GroupEditing},
	{Action: "undo", Title: "Undo", Accels: []string{"<Control>z"}, Scope: ScopeApp, Group: GroupEditing},
	{Action: "redo", Title: "Redo", Accels: []string{"<Control><Shift>z", "<Control>y"}, Scope: ScopeApp, Group: GroupEditing},
	{Action: "clear-done", Title: "Clear done (always confirm)", Accels: []string{"<Control><Shift>BackSpace"}, Scope: ScopeApp, Group: GroupEditing},

	{Action: "copy", Title: "Copy", Accels: []string{"<Control>c"}, Scope: ScopeList, Group: GroupClipboard},
	{Action: "copy-as-list", Title: "Copy as List", Accels: []string{"<Control><Shift>c"}, Scope: ScopeList, Group: GroupClipboard},

	{Action: "switch-project", Title: "Switch to project 1-9", Accels: []string{
		"<Control>1", "<Control>2", "<Control>3", "<Control>4", "<Control>5",
		"<Control>6", "<Control>7", "<Control>8", "<Control>9",
	}, Scope: ScopeApp, Group: GroupProjects},
	{Action: "next-project", Title: "Next project", Accels: []string{"<Control>Tab"}, Scope: ScopeApp, Group: GroupProjects},
	{Action: "previous-project", Title: "Previous project", Accels: []string{"<Control><Shift>Tab"}, Scope: ScopeApp, Group: GroupProjects},

	{Action: "shortcuts", Title: "Keyboard shortcuts", Accels: []string{"<Control>question"}, Scope: ScopeApp, Group: GroupWindow},
	{Action: "hide-panel", Title: "Hide panel (does not quit)", Accels: []string{"<Control>w"}, Scope: ScopeApp, Group: GroupWindow},
	{Action: "quit", Title: "Quit", Accels: []string{"<Control>q"}, Scope: ScopeApp, Group: GroupWindow},
}

// AvoidList is the exact set of bindings Table must never use, because
// each collides with GTK's or IBus's own use of that key: Ctrl+Shift+U
// (IBus Unicode entry), Ctrl+Shift+E (ibus-table), Ctrl+Shift+I and
// Ctrl+Shift+D (GTK Inspector), F10 (GTK menu key), Ctrl+. and Ctrl+;
// (GTK emoji chooser inside text widgets), Ctrl+M (GTK Text treats it
// as Return in some IMs), and Alt+Left/Alt+Right (some tiling setups
// and Plasma layouts bind these).
var AvoidList = []string{
	"<Control><Shift>u",
	"<Control><Shift>e",
	"<Control><Shift>i",
	"<Control><Shift>d",
	"F10",
	"<Control>period",
	"<Control>semicolon",
	"<Control>m",
	"<Alt>Left",
	"<Alt>Right",
}

// ByAction returns the Entry for action, and whether it was found.
func ByAction(action string) (Entry, bool) {
	for _, e := range Table {
		if e.Action == action {
			return e, true
		}
	}
	return Entry{}, false
}

// ApplyOverrides replaces the Accels of Table entries named in
// overrides (action -> a single accelerator string, config.toml's
// [keys] section) with that one accelerator, one override at a time —
// each is kept only if it parses as a real gtk.AcceleratorParse
// accelerator, names an action whose Scope isn't ScopeGlobal (a global
// binding is evdev/hotkey-driven, never a GTK accelerator, so there is
// nothing for a "GTK accelerator override" to change — accepting one
// would only corrupt the generated shortcuts window with a chip that
// names no real binding), and does not collide with another binding
// already active in the same scope: Resolve's own index build assumes
// Table never collides within a scope (see
// TestNoAccelCollisionsWithinScope), so letting a colliding override
// through would panic on the very next keypress. An override naming an
// action Table has no entry for is silently skipped — that's
// config.Load-vs-keymap.ByAction's job to warn about, not this
// function's.
//
// Overrides are applied in sorted-by-action order for a deterministic
// outcome when two user overrides collide with each other (the first,
// alphabetically, wins; the second is rejected as a collision) — map
// iteration order is otherwise randomized per Go's own spec, which
// would make that outcome flip from run to run.
//
// Call once, at startup, before Resolve or NewShortcutsWindow read
// Table: both are generated from it fresh on every call, so an override
// applied later would still take effect, just not retroactively for
// whatever already ran.
//
// Returns the rejected subset as action -> reason; every override not
// named here was applied.
func ApplyOverrides(overrides map[string]string) map[string]string {
	actions := make([]string, 0, len(overrides))
	for action := range overrides {
		actions = append(actions, action)
	}
	sort.Strings(actions)

	rejected := make(map[string]string)
	for _, action := range actions {
		accel := overrides[action]
		if accel == "" {
			continue
		}

		i := -1
		var scope Scope
		for j, e := range Table {
			if e.Action == action {
				i, scope = j, e.Scope
				break
			}
		}
		if i < 0 {
			continue
		}
		if scope == ScopeGlobal {
			rejected[action] = "a global binding is not a GTK accelerator and cannot be overridden here"
			continue
		}
		if _, _, ok := gtk.AcceleratorParse(accel); !ok {
			rejected[action] = "not a valid accelerator"
			continue
		}

		prev := Table[i].Accels
		Table[i].Accels = []string{accel}
		if _, err := index(scope); err != nil {
			Table[i].Accels = prev
			rejected[action] = err.Error()
		}
	}
	return rejected
}
