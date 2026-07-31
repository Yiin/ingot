// Package searchbar builds the panel's search field and its trailing
// overflow button: a 30dp pill holding a magnifier icon, a bare GtkText,
// and a live match-count label, followed by an 8dp gap and a 30dp circular
// MenuButton.
package searchbar

import (
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// fieldInset is the search field's own left/right inset for the icon and
// match-count label — 10dp per the spec, distinct from --search-gap (8dp),
// which is the gap between the field and the overflow button.
const fieldInset = 10

// SearchBar is the pill-shaped search field plus its overflow button.
//
// It only renders the query and reports it back through OnQueryChanged; it
// runs no search itself. The caller (copper-l2z.28) drives matching and
// reports the result count back through SetMatchCount.
type SearchBar struct {
	root     *gtk.Box
	entry    *gtk.Text
	count    *gtk.Label
	overflow *gtk.MenuButton

	onQueryChanged  func(query string)
	onEscapeAtEmpty func()
}

// New builds the search bar. Call Widget to place it in a container.
func New() *SearchBar {
	s := &SearchBar{}

	field := gtk.NewBox(gtk.OrientationHorizontal, 6)
	field.AddCSSClass("search-field")
	field.SetHExpand(true)

	icon := gtk.NewImageFromIconName("system-search-symbolic")
	icon.SetPixelSize(16)
	icon.SetMarginStart(fieldInset)
	icon.AddCSSClass("search-icon")

	s.entry = gtk.NewText()
	s.entry.SetPlaceholderText("Search")
	s.entry.SetHExpand(true)
	s.entry.AddCSSClass("search-entry")
	s.entry.ConnectChanged(func() { s.handleQueryChanged() })

	s.count = gtk.NewLabel("")
	s.count.AddCSSClass("search-match-count")
	s.count.SetVisible(false)
	s.count.SetMarginEnd(fieldInset)

	field.Append(icon)
	field.Append(s.entry)
	field.Append(s.count)

	s.overflow = gtk.NewMenuButton()
	s.overflow.SetIconName("view-more-symbolic")
	// Deliberately not SetLabel("..."): with the icon already set, SetLabel
	// renders a stray dropdown arrow next to it — verified in the spec.
	s.overflow.SetAlwaysShowArrow(false)
	s.overflow.AddCSSClass("overflow-btn")

	s.root = gtk.NewBox(gtk.OrientationHorizontal, theme.SearchGap)
	s.root.Append(field)
	s.root.Append(s.overflow)

	s.installShortcuts()

	return s
}

// Widget returns the root widget to place in the panel.
func (s *SearchBar) Widget() gtk.Widgetter { return s.root }

// OverflowButton returns the overflow MenuButton so the menus package
// (copper-l2z.23) can attach its popover or menu model.
func (s *SearchBar) OverflowButton() *gtk.MenuButton { return s.overflow }

// Focus grabs keyboard focus into the search entry.
func (s *SearchBar) Focus() { s.entry.GrabFocus() }

// Clear empties the query, e.g. for the panel's "no matches" empty
// state's Clear search button (copper-l2z.26). It fires the same
// ConnectChanged path a user clearing the field by hand would, so
// OnQueryChanged still runs.
func (s *SearchBar) Clear() { s.entry.SetText("") }

// OnQueryChanged registers f to be called with the current query text
// every time it changes.
func (s *SearchBar) OnQueryChanged(f func(query string)) { s.onQueryChanged = f }

// OnEscapeAtEmpty registers f to be called when Escape is pressed while the
// query is already empty — the second Escape in "clears then blurs". The
// panel (copper-l2z.26) wires this to move focus into the composer.
func (s *SearchBar) OnEscapeAtEmpty(f func()) { s.onEscapeAtEmpty = f }

// SetMatchCount sets the live match count shown as trailing muted text.
// It is only visible while the query is non-empty.
func (s *SearchBar) SetMatchCount(n int) {
	s.count.SetText(strconv.Itoa(n))
	s.updateCountVisibility()
}

func (s *SearchBar) handleQueryChanged() {
	s.updateCountVisibility()
	if s.onQueryChanged != nil {
		s.onQueryChanged(s.entry.Text())
	}
}

func (s *SearchBar) updateCountVisibility() {
	s.count.SetVisible(s.entry.Text() != "")
}

// installShortcuts wires Ctrl+F (global scope, so it fires regardless of
// where focus currently sits inside the panel) and Escape (local to the
// entry: clears a non-empty query, or reports to onEscapeAtEmpty once
// already empty).
func (s *SearchBar) installShortcuts() {
	focusTrigger := gtk.NewKeyvalTrigger(gdk.KEY_f, gdk.ControlMask)
	focusAction := gtk.NewCallbackAction(func(gtk.Widgetter, *glib.Variant) bool {
		s.Focus()
		return true
	})
	shortcuts := gtk.NewShortcutController()
	shortcuts.SetScope(gtk.ShortcutScopeGlobal)
	shortcuts.AddShortcut(gtk.NewShortcut(focusTrigger, focusAction))
	s.root.AddController(shortcuts)

	escapeCtrl := gtk.NewEventControllerKey()
	escapeCtrl.ConnectKeyPressed(func(keyval, _ uint, _ gdk.ModifierType) bool {
		if keyval != gdk.KEY_Escape {
			return false
		}
		if s.entry.Text() != "" {
			s.entry.SetText("")
			return true
		}
		if s.onEscapeAtEmpty != nil {
			s.onEscapeAtEmpty()
		}
		return true
	})
	s.entry.AddController(escapeCtrl)
}
