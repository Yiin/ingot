//go:build integration

// These tests need a real GDK display connection (gtk.Init +
// gdk.DisplayGetDefault), so they are gated behind the integration tag —
// same convention as internal/ui/theme/display_test.go and
// internal/ui/notelist/list_integration_test.go — and need
// copper-l2z.31's headless sway harness (WLR_BACKENDS=headless,
// GSK_RENDERER=cairo) to actually run. This worktree has no display at
// all: these tests only need to compile here (go vet -tags integration),
// never execute.
//
// Driving a synthetic Return/Escape key-press needs a mapped, focused
// surface, so — following internal/ui/composer's own precedent — the
// whitespace-only-commit reject flash and the Escape cascade are not
// exercised here either; this file only checks that Shell wires the
// real composer/search instances into its tree, structurally.
package panel

import (
	"strings"
	"sync"
	"testing"
	"time"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/notelist"
	"github.com/Yiin/ingot/internal/ui/theme"
	"github.com/Yiin/ingot/internal/ui/toast"
)

var gtkInitOnce sync.Once

// requireDisplay initialises GTK once per test binary and returns the
// default display, skipping the test if none is reachable.
func requireDisplay(t *testing.T) *gdk.Display {
	t.Helper()
	gtkInitOnce.Do(gtk.Init)
	display := gdk.DisplayGetDefault()
	if display == nil {
		t.Skip("no GDK display available")
	}
	return display
}

// pump drains the default main context so idle/timeout callbacks run
// synchronously instead of waiting for a real event loop — same idiom as
// internal/ui/notelist's own pump().
func pump() {
	ctx := glib.MainContextDefault()
	for ctx.Iteration(false) {
	}
}

// pumpUntilMapped shows win and drains the main context until the
// compositor actually maps it. Shell's notelist.List binds no rows/
// headers, and no widget in win's tree gets a real AllocatedHeight or
// keyboard focus, until win is mapped — a bare pump() right after
// SetChild only drains whatever's already queued on an unmapped window.
// Bounded so a genuine headless-harness failure to map fails fast
// instead of hanging.
func pumpUntilMapped(t *testing.T, win *gtk.Window) {
	t.Helper()
	win.SetVisible(true)
	ctx := glib.MainContextDefault()
	deadline := time.Now().Add(5 * time.Second)
	for {
		for ctx.Iteration(false) {
		}
		if win.Mapped() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("window did not map within 5s")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Mapped() flipping true only means the surface entered the map
	// state, not that GtkListView has run the idle callback that binds
	// its visible range against the now-known viewport size — same
	// "give the frame clock room to actually settle" idiom as
	// internal/ui/panel's own screenshot test (pumpFor).
	settle := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(settle) {
		for ctx.Iteration(false) {
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// recordingNotifier is a fake toast.Notifier that records every call,
// for asserting NotifyEmptySelection/NotifyDuplicate reach the HUD
// without needing a live layer-shell surface.
type recordingNotifier struct {
	captured []string
	messages []string
}

func (n *recordingNotifier) Captured(text string) { n.captured = append(n.captured, text) }
func (n *recordingNotifier) Message(text string)  { n.messages = append(n.messages, text) }

var _ toast.Notifier = (*recordingNotifier)(nil)

// walk visits w and every descendant, depth-first, via GTK's own widget
// tree (FirstChild/NextSibling) — the only way to inspect internal/ui/
// notelist's recycled row/header widgets from outside that package.
func walk(w gtk.Widgetter, visit func(gtk.Widgetter)) {
	if w == nil {
		return
	}
	visit(w)
	base := gtk.BaseWidget(w)
	for c := base.FirstChild(); c != nil; c = gtk.BaseWidget(c).NextSibling() {
		walk(c, visit)
	}
}

// countVisibleClass counts descendants of root carrying class and
// currently visible (a hidden section header, per header.go's
// empty-title contract, must not count).
func countVisibleClass(root gtk.Widgetter, class string) int {
	n := 0
	walk(root, func(w gtk.Widgetter) {
		b := gtk.BaseWidget(w)
		if b.HasCSSClass(class) && b.Visible() {
			n++
		}
	})
	return n
}

// countEllipsizedLabels counts every descendant GtkLabel whose Pango
// layout actually ellipsized — only a note row's clamped Label ever
// enables ellipsization (widget.NewLabel), so this is a reliable proxy
// for "how many rows are visibly truncated."
func countEllipsizedLabels(root gtk.Widgetter) int {
	n := 0
	walk(root, func(w gtk.Widgetter) {
		if lbl, ok := w.(*gtk.Label); ok && lbl.Layout().IsEllipsized() {
			n++
		}
	})
	return n
}

// findWithClass returns the first descendant of root carrying class, or
// nil.
func findWithClass(root gtk.Widgetter, class string) gtk.Widgetter {
	var found gtk.Widgetter
	walk(root, func(w gtk.Widgetter) {
		if found != nil {
			return
		}
		if gtk.BaseWidget(w).HasCSSClass(class) {
			found = w
		}
	})
	return found
}

// isDescendant reports whether target is w or lies somewhere under it,
// compared by raw GObject pointer identity via coreglib.InternObject(x).
// Native() — gotk4 hands back a distinct Go wrapper per traversal step,
// so a plain == on the gtk.Widgetter interface value is not reliable,
// and gtk.Widget's own Native() is a different concept entirely (the
// nearest GtkNative ancestor, e.g. the enclosing window — the same
// value for every widget in the tree once mapped, per the epic's own
// gotcha note on this exact pitfall).
func isDescendant(root, target gtk.Widgetter) bool {
	if target == nil {
		return false
	}
	want := coreglib.InternObject(target).Native()
	found := false
	walk(root, func(w gtk.Widgetter) {
		if coreglib.InternObject(w).Native() == want {
			found = true
		}
	})
	return found
}

func newFixtureSections() []notelist.Section {
	return []notelist.Section{
		{ID: "inbox", Title: "Inbox"},
		{ID: "work", Title: "Work"},
		{ID: "ideas", Title: "Ideas"},
	}
}

// TestShellRendersFixtureFromSpec is the acceptance criterion verbatim:
// three section headers, exactly one truncated row, two rows selectable
// together, and a focused composer.
func TestShellRendersFixtureFromSpec(t *testing.T) {
	display := requireDisplay(t)
	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	s := New(newFixtureSections(), "Notes", toast.NewInPanel(), toast.Nop{})

	longWord := strings.Repeat("x", 200)
	a := notelist.NewItem("1", "inbox", "a short note", false)
	b := notelist.NewItem("2", "inbox", longWord, false)
	c := notelist.NewItem("3", "work", "another note", false)
	s.List().Model().AppendAll([]*notelist.Item{a, b, c})
	// AppendAll doesn't drive the hint/list-visibility switch itself —
	// see RefreshEmptyState's doc comment — so without this the shell
	// keeps showing the first-run hint over a list that now has real
	// rows behind it, and a hidden GtkListView binds nothing.
	s.RefreshEmptyState("", 0)

	win := gtk.NewWindow()
	win.SetChild(s.Widget())
	win.SetDefaultSize(theme.PanelWidth, theme.PanelHeight)
	pumpUntilMapped(t, win)

	if got := countVisibleClass(win, "section-header"); got != 3 {
		t.Errorf("visible section headers = %d, want 3", got)
	}
	if got := countEllipsizedLabels(win); got != 1 {
		t.Errorf("truncated rows = %d, want 1", got)
	}

	s.List().SelectItems([]*notelist.Item{a, c})
	pump()
	if got := s.List().Selection().Selection().Size(); got != 2 {
		t.Errorf("selected rows = %d, want 2", got)
	}

	s.Composer().Focus()
	pump()
	focused := win.Focus()
	if focused == nil || !isDescendant(s.Composer().Widget(), focused) {
		t.Errorf("composer is not focused after Composer().Focus()")
	}
}

// TestLongSingleWordRowMeasures70dp covers the acceptance criterion's
// last sentence: a 200-character single token wraps mid-word and its row
// still measures 70dp.
func TestLongSingleWordRowMeasures70dp(t *testing.T) {
	display := requireDisplay(t)
	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	s := New([]notelist.Section{{ID: "a", Title: "A"}}, "Notes", toast.NewInPanel(), toast.Nop{})
	s.List().Model().Append(notelist.NewItem("1", "a", strings.Repeat("x", 200), false))
	s.RefreshEmptyState("", 0)

	win := gtk.NewWindow()
	win.SetChild(s.Widget())
	win.SetDefaultSize(theme.PanelWidth, theme.PanelHeight)
	pumpUntilMapped(t, win)

	row := findWithClass(win, "note-card")
	if row == nil {
		t.Fatalf("no .note-card row found")
	}
	// gtk_widget_measure folds a widget's own CSS margin into its
	// reported min/natural size (unlike the web CSS box model, where
	// margin never counts toward an element's own offsetHeight) — so
	// AllocatedHeight() here includes .note-card's margin-top
	// (theme.CardGap, the gap before this row), on top of its visual
	// content+padding box. Subtract it to get the card's own 70dp.
	if h := gtk.BaseWidget(row).AllocatedHeight() - theme.CardGap; h != 70 {
		t.Errorf("row AllocatedHeight (minus CardGap margin) = %d, want 70", h)
	}
}

// TestFirstRunShowsHintAndFocusesComposer covers "never show a truly
// empty shell": zero notes anywhere shows the centred hint and focuses
// the composer.
func TestFirstRunShowsHintAndFocusesComposer(t *testing.T) {
	display := requireDisplay(t)
	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	s := New([]notelist.Section{{ID: "inbox", Title: "Inbox"}}, "Notes", toast.NewInPanel(), toast.Nop{})

	win := gtk.NewWindow()
	win.SetChild(s.Widget())
	win.SetDefaultSize(theme.PanelWidth, theme.PanelHeight)
	pumpUntilMapped(t, win)

	if !s.hint.Visible() {
		t.Errorf("hint block is not visible on a genuinely empty project")
	}
	if s.list.ListView().Visible() {
		t.Errorf("list view should stay hidden behind the hint while empty")
	}
	focused := win.Focus()
	if focused == nil || !isDescendant(s.Composer().Widget(), focused) {
		t.Errorf("composer is not focused on first run")
	}
}

// TestEmptySectionShowsPlaceholder covers "empty section": a declared
// section with zero notes still renders its own placeholder card.
func TestEmptySectionShowsPlaceholder(t *testing.T) {
	display := requireDisplay(t)
	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	s := New([]notelist.Section{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}}, "Notes", toast.NewInPanel(), toast.Nop{})
	s.List().Model().Append(notelist.NewItem("1", "a", "one note", false))
	s.RefreshEmptyState("", 0)

	win := gtk.NewWindow()
	win.SetChild(s.Widget())
	win.SetDefaultSize(theme.PanelWidth, theme.PanelHeight)
	pumpUntilMapped(t, win)

	if got := countVisibleClass(win, "note-placeholder"); got != 1 {
		t.Errorf("visible placeholder cards = %d, want 1 (section B only)", got)
	}
}

// TestProjectWithNoSectionsRendersHeaderless covers "a project with no
// sections": the implicit unnamed section (Title: "") renders its notes
// with no header and no rule.
func TestProjectWithNoSectionsRendersHeaderless(t *testing.T) {
	display := requireDisplay(t)
	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	s := New([]notelist.Section{{ID: "", Title: ""}}, "Notes", toast.NewInPanel(), toast.Nop{})
	s.List().Model().Append(notelist.NewItem("1", "", "flush under search", false))

	win := gtk.NewWindow()
	win.SetChild(s.Widget())
	win.SetDefaultSize(theme.PanelWidth, theme.PanelHeight)
	pump()

	if got := countVisibleClass(win, "section-header"); got != 0 {
		t.Errorf("visible section headers = %d, want 0 for an unnamed section", got)
	}
}

// TestSearchNoMatchesHidesListAndShowsClearBlock covers "search with no
// matches": RefreshEmptyState(query, 0) on an otherwise non-empty
// project hides the list and shows the "no matches" block; clearing
// through the search bar and re-running with an empty query restores it.
func TestSearchNoMatchesHidesListAndShowsClearBlock(t *testing.T) {
	display := requireDisplay(t)
	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	s := New([]notelist.Section{{ID: "a", Title: "A"}}, "Notes", toast.NewInPanel(), toast.Nop{})
	s.List().Model().Append(notelist.NewItem("1", "a", "a real note", false))

	win := gtk.NewWindow()
	win.SetChild(s.Widget())
	win.SetDefaultSize(theme.PanelWidth, theme.PanelHeight)
	pump()

	s.RefreshEmptyState("xyzzy", 0)
	if s.list.ListView().Visible() {
		t.Errorf("list should hide while search has zero matches")
	}
	if !s.searchEmpty.Visible() {
		t.Errorf("search-empty block should show while search has zero matches")
	}
	if got := s.searchEmptyLabel.Text(); got != `No notes match "xyzzy"` {
		t.Errorf("search-empty label = %q, want the query quoted", got)
	}

	s.search.Clear()
	s.RefreshEmptyState("", 0)
	if !s.list.ListView().Visible() {
		t.Errorf("list should reappear once the query clears")
	}
	if s.searchEmpty.Visible() {
		t.Errorf("search-empty block should hide once the query clears")
	}
}

// TestNotifyEmptySelectionShowsHUD covers "capture with an empty
// selection": no note is created (Shell has no store dependency to
// create one through in the first place) and the dark HUD reads
// "Nothing selected".
func TestNotifyEmptySelectionShowsHUD(t *testing.T) {
	requireDisplay(t)

	n := &recordingNotifier{}
	s := New([]notelist.Section{{ID: "a", Title: "A"}}, "Notes", toast.NewInPanel(), n)

	s.NotifyEmptySelection()

	if len(n.captured) != 1 || n.captured[0] != "Nothing selected" {
		t.Errorf("Captured calls = %v, want exactly [\"Nothing selected\"]", n.captured)
	}
}

// TestNotifyDuplicateFlashesRowAndShowsHUD covers "capture duplicating
// the newest note": the existing row's ring flashes and the dark HUD
// reads "Already captured".
func TestNotifyDuplicateFlashesRowAndShowsHUD(t *testing.T) {
	display := requireDisplay(t)
	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	n := &recordingNotifier{}
	s := New([]notelist.Section{{ID: "a", Title: "A"}}, "Notes", toast.NewInPanel(), n)
	it := notelist.NewItem("1", "a", "the newest note", false)
	s.List().Model().Append(it)
	s.RefreshEmptyState("", 0)

	win := gtk.NewWindow()
	win.SetChild(s.Widget())
	win.SetDefaultSize(theme.PanelWidth, theme.PanelHeight)
	pumpUntilMapped(t, win)

	s.NotifyDuplicate(it)

	if got := countVisibleClass(win, "duplicate-flash"); got != 1 {
		t.Errorf("rows carrying .duplicate-flash = %d, want 1", got)
	}
	if len(n.captured) != 1 || n.captured[0] != "Already captured" {
		t.Errorf("Captured calls = %v, want exactly [\"Already captured\"]", n.captured)
	}
}

// TestUnfocusedStyleTogglesOnlyPanelClass covers "unfocused panel
// state": SetFocused only ever toggles the one CSS class the stylesheet
// keys its ring-dim/shadow-halve rule off — see
// TestUnfocusedRuleTouchesOnlyRingAndShadow in css_test.go for the
// stylesheet side of this contract.
func TestUnfocusedStyleTogglesOnlyPanelClass(t *testing.T) {
	requireDisplay(t)

	s := New([]notelist.Section{{ID: "a", Title: "A"}}, "Notes", toast.NewInPanel(), toast.Nop{})

	if s.root.HasCSSClass("unfocused") {
		t.Fatalf("panel starts unfocused; want focused by default")
	}
	s.SetFocused(false)
	if !s.root.HasCSSClass("unfocused") {
		t.Errorf("SetFocused(false) did not add .unfocused")
	}
	s.SetFocused(true)
	if s.root.HasCSSClass("unfocused") {
		t.Errorf("SetFocused(true) did not remove .unfocused")
	}
}

// TestShellClampsHeightOnShortWorkArea covers "a panel taller than the
// work area": inside a window shorter than the panel's natural height,
// the search field and composer stay pinned at their full size while the
// list area is what shrinks.
func TestShellClampsHeightOnShortWorkArea(t *testing.T) {
	display := requireDisplay(t)
	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	s := New(newFixtureSections(), "Notes", toast.NewInPanel(), toast.Nop{})
	for i := 0; i < 30; i++ {
		s.List().Model().Append(notelist.NewItem("n", "inbox", "a note", false))
	}

	const shortHeight = 220
	win := gtk.NewWindow()
	win.SetChild(s.Widget())
	win.SetDefaultSize(theme.PanelWidth, shortHeight)
	pumpUntilMapped(t, win)

	if h := gtk.BaseWidget(win).AllocatedHeight(); h > shortHeight {
		t.Fatalf("window allocated %d, want <= %d", h, shortHeight)
	}
	if h := gtk.BaseWidget(s.SearchBar().Widget()).AllocatedHeight(); h < theme.SearchHeight {
		t.Errorf("search bar AllocatedHeight = %d, want >= %d (must stay pinned)", h, theme.SearchHeight)
	}
	if h := gtk.BaseWidget(s.Composer().Widget()).AllocatedHeight(); h < theme.ComposerMinHeight {
		t.Errorf("composer AllocatedHeight = %d, want >= %d (must stay pinned)", h, theme.ComposerMinHeight)
	}
}

// TestFencedCodeBlockAndMultiLineNoteRenderWithoutPanicking covers "a
// note with a fenced code block" and "a multi-line note": rendering is
// internal/ui/mdpango and internal/ui/widget's own contract (see their
// packages' tests), so this only exercises the full panel assembly path
// end to end with both bodies, guarding against a wiring regression.
func TestFencedCodeBlockAndMultiLineNoteRenderWithoutPanicking(t *testing.T) {
	display := requireDisplay(t)
	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	s := New([]notelist.Section{{ID: "a", Title: "A"}}, "Notes", toast.NewInPanel(), toast.Nop{})
	s.List().Model().AppendAll([]*notelist.Item{
		notelist.NewItem("1", "a", "before\n\n\n\nafter, four blank lines collapsed", false),
		notelist.NewItem("2", "a", "text\n\n```go\nfunc f() {}\n```\n\nmore text", false),
	})
	s.RefreshEmptyState("", 0)

	win := gtk.NewWindow()
	win.SetChild(s.Widget())
	win.SetDefaultSize(theme.PanelWidth, theme.PanelHeight)
	pumpUntilMapped(t, win)

	if got := countVisibleClass(win, "note-card"); got != 2 {
		t.Errorf("visible note cards = %d, want 2", got)
	}
}
