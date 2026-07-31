package panel

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/composer"
	"github.com/Yiin/ingot/internal/ui/notelist"
	"github.com/Yiin/ingot/internal/ui/search"
	"github.com/Yiin/ingot/internal/ui/searchbar"
	"github.com/Yiin/ingot/internal/ui/theme"
	"github.com/Yiin/ingot/internal/ui/toast"
)

// Shell is the assembled panel: search bar, note list and composer
// stacked inside the opaque rounded card, plus the empty/edge-state
// overlays this child spec adds on top of them — see the package doc.
//
// An implicit unnamed section (a project with no declared sections) is
// expressed the same way every section is: pass
// notelist.Section{ID: "", Title: ""} to New or List().Model().
// SetSections. internal/ui/notelist hides that section's own header and
// rule whenever its title is empty, landing its notes flush under the
// search field.
type Shell struct {
	outer *gtk.Overlay // root: toast overlay -> root (below) -> [search, stage, composer]
	root  *gtk.Box
	stage *gtk.Overlay // list -> [hint, search-empty] overlays

	search    *searchbar.SearchBar
	searchCtl *search.Controller
	list      *notelist.List
	composer  *composer.Composer
	notifier  toast.Notifier
	notice    *gtk.Label

	hint             *gtk.Box
	searchEmpty      *gtk.Box
	searchEmptyLabel *gtk.Label

	lastState state

	onFilterChanged func()
}

// New assembles the panel shell over sections (see the implicit-
// unnamed-section note above) with a composer placeholder naming
// project.
//
// panelToast is the in-panel light toast the caller's toast.Toaster
// already owns (Toaster.Panel()) — Shell embeds that exact widget,
// tracking its bottom inset as the composer grows, rather than building
// its own, so the app-level Toaster's Message calls actually reach
// something visible.
//
// notifier drives the dark HUD behind NotifyEmptySelection and
// NotifyDuplicate; pass toast.Nop{} where no HUD is wanted (most tests).
func New(sections []notelist.Section, project string, panelToast *toast.InPanel, notifier toast.Notifier) *Shell {
	s := &Shell{
		list:     notelist.New(sections),
		search:   searchbar.New(),
		composer: composer.New(project),
		notifier: notifier,
	}

	s.search.OnEscapeAtEmpty(func() { s.composer.Focus() })

	s.searchCtl = search.New(s.list)
	s.search.OnQueryChanged(func(query string) {
		n := s.searchCtl.Apply(query)
		s.search.SetMatchCount(n)
		s.RefreshEmptyState(query, n)
		if s.onFilterChanged != nil {
			s.onFilterChanged()
		}
	})

	s.hint = newHintBlock()
	s.searchEmpty, s.searchEmptyLabel = newSearchEmptyBlock(func() {
		// Clear's ConnectChanged fires OnQueryChanged synchronously,
		// which re-runs the search and refreshes the empty state itself
		// — no separate call needed here.
		s.search.Clear()
	})

	s.stage = gtk.NewOverlay()
	s.stage.SetChild(s.list)
	s.stage.AddOverlay(s.hint)
	s.stage.SetMeasureOverlay(s.hint, false)
	s.stage.SetClipOverlay(s.hint, false)
	s.stage.AddOverlay(s.searchEmpty)
	s.stage.SetMeasureOverlay(s.searchEmpty, false)
	s.stage.SetClipOverlay(s.searchEmpty, false)

	s.notice = gtk.NewLabel("")
	s.notice.AddCSSClass("panel-notice")
	s.notice.SetWrap(true)
	s.notice.SetXAlign(0)
	s.notice.SetVisible(false)

	s.root = gtk.NewBox(gtk.OrientationVertical, 0)
	s.root.AddCSSClass("ingot-panel")
	s.root.SetSizeRequest(theme.PanelWidth, -1)
	s.root.Append(s.notice)
	s.root.Append(s.search.Widget())
	s.root.Append(s.stage)
	s.root.Append(s.composer.Widget())

	// The toast overlay wraps the whole panel, not just the list stage:
	// PanelToastGap is measured as clearance above the panel's own
	// bottom edge (see theme.PanelToastGap's doc comment), and
	// SetBottomInset(toastBottomInset(...)) only lands the toast just
	// above the composer if the overlay's own bottom coincides with the
	// composer's — copper-l2z.24's gotcha note for this child.
	s.outer = gtk.NewOverlay()
	s.outer.SetChild(s.root)
	s.outer.AddOverlay(panelToast.Widget())
	s.outer.SetMeasureOverlay(panelToast.Widget(), false)
	s.outer.SetClipOverlay(panelToast.Widget(), false)

	panelToast.SetBottomInset(toastBottomInset(theme.ComposerMinHeight))
	s.composer.OnHeightChanged(func(h int) {
		panelToast.SetBottomInset(toastBottomInset(h))
	})

	// grab_focus silently fails on an unrooted widget, and this next
	// call can run before Widget() is ever placed in a window — retry
	// the first-run composer focus once the shell actually gets a
	// GtkRoot (realize fires on gtk.Widget.Realize or first map, i.e.
	// whenever internal/layershell or a test finally roots it).
	s.root.ConnectRealize(func() {
		if s.lastState == stateHint {
			s.composer.Focus()
		}
	})

	s.RefreshEmptyState("", 0)

	return s
}

// toastBottomInset is SetBottomInset's own arithmetic, split out so it
// is unit-testable without a GTK display: composerContentHeight is the
// scroll-content height composer.OnHeightChanged reports, which sits
// inside two more layers of padding before reaching the panel's true
// outer bottom edge — the composer's own top/bottom CSS padding
// (2*theme.CardPadY) and the panel's own bottom padding
// (theme.PanelPadBottom) — before adding PanelToastGap's clearance
// above the composer's top edge.
func toastBottomInset(composerContentHeight int) int {
	return composerContentHeight + 2*theme.CardPadY + theme.PanelPadBottom + theme.PanelToastGap
}

// Widget returns the panel's root widget, ready for internal/layershell
// (copper-l2z.19) to place inside the layer-shell surface's window.
func (s *Shell) Widget() gtk.Widgetter { return s.outer }

// SearchBar returns the panel's search bar, for a later child
// (copper-l2z.28's live filtering, copper-l2z.30's wiring) to attach
// real matching and its overflow menu.
func (s *Shell) SearchBar() *searchbar.SearchBar { return s.search }

// List returns the panel's note list, for a later child to mutate
// through its Model() and wire selection/keyboard behaviour against.
func (s *Shell) List() *notelist.List { return s.list }

// OnFilterChanged registers f to run after every live-search filter
// change (a query keystroke, or Clear) has been applied to the list.
// SetFilter changes which rows are displayed with no store event to
// hang off of, so a caller that keeps its own idea of the list's
// display order (copper-l2z.61's keymap.Nav wiring) needs this hook to
// stay in sync — without it, Nav's row order goes stale the moment a
// search query changes what's actually visible.
func (s *Shell) OnFilterChanged(f func()) { s.onFilterChanged = f }

// Composer returns the panel's composer, for a later child to wire
// OnCommit against note creation.
func (s *Shell) Composer() *composer.Composer { return s.composer }

// SetFocused toggles the panel's focused/unfocused visual state: while
// unfocused, the focus-ring family (the *:focus-visible outline,
// .note-card.selected/.selection-anchor, .composer.focused) dims to 45%
// opacity and the panel shadow halves, via the .unfocused CSS class —
// every other colour (fills, text, done state) is untouched.
func (s *Shell) SetFocused(focused bool) {
	if focused {
		s.root.RemoveCSSClass("unfocused")
	} else {
		s.root.AddCSSClass("unfocused")
	}
}
