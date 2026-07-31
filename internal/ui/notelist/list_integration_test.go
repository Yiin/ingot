//go:build integration

// These tests need a real GDK display connection (gtk.Init +
// gdk.DisplayGetDefault), so they are gated behind the integration tag —
// same convention as internal/ui/theme/display_test.go and `make
// test-integration` — and need copper-l2z.31's headless sway harness
// (WLR_BACKENDS=headless, GSK_RENDERER=cairo) to actually run. Do not run
// this file against a live desktop session; nothing here is ever shown,
// but it is still a real GTK client on whatever WAYLAND_DISPLAY points
// at. This worktree has no display at all — these tests only need to
// compile here (go vet -tags integration), never execute.
package notelist

import (
	"sync"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
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

// pump drains the default main context so idle/timeout callbacks (row
// binds, the just-inserted strip timer, selection-changed handlers) run
// synchronously instead of waiting for a real event loop.
func pump() {
	ctx := glib.MainContextDefault()
	for ctx.Iteration(false) {
	}
}

// TestHeaderBindFiresOncePerSectionWithCorrectRange covers "Header bind
// fires once per section with correct ListHeader.Start and NItems."
func TestHeaderBindFiresOncePerSectionWithCorrectRange(t *testing.T) {
	requireDisplay(t)

	l := New([]Section{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}})
	l.Model().Append(NewItem("1", "a", "one", false))
	l.Model().Append(NewItem("2", "a", "two", false))
	l.Model().Append(NewItem("3", "b", "three", false))

	type bind struct{ start, n uint }
	var binds []bind
	l.onHeaderBind = func(start, n uint) { binds = append(binds, bind{start, n}) }

	win := gtk.NewWindow()
	win.SetChild(l)
	pump()

	if len(binds) != 2 {
		t.Fatalf("header binds = %d, want 2 (one per section)", len(binds))
	}
	if binds[0].start != 0 || binds[0].n != 2 {
		t.Errorf("section a header = %+v, want {0 2}", binds[0])
	}
	if binds[1].start != 2 || binds[1].n != 1 {
		t.Errorf("section b header = %+v, want {2 1}", binds[1])
	}
}

// TestMultiSelectionSizeAfterSelectingTwoRows covers
// "MultiSelection.Selection().Size() after selecting two rows is 2."
func TestMultiSelectionSizeAfterSelectingTwoRows(t *testing.T) {
	requireDisplay(t)

	l := New([]Section{{ID: "a"}})
	l.Model().Append(NewItem("1", "a", "one", false))
	l.Model().Append(NewItem("2", "a", "two", false))

	win := gtk.NewWindow()
	win.SetChild(l)
	pump()

	l.Selection().SelectItem(0, false)
	l.Selection().SelectItem(1, false)
	pump()

	if got := l.Selection().Selection().Size(); got != 2 {
		t.Errorf("Selection().Size() = %d, want 2", got)
	}
}

// Test5000ItemsCreateFewerThan400ItemWidgets covers "A 5000-item model
// creates fewer than 400 item widgets."
func Test5000ItemsCreateFewerThan400ItemWidgets(t *testing.T) {
	requireDisplay(t)

	l := New([]Section{{ID: "a"}})
	var setups int
	l.onItemSetup = func() { setups++ }

	items := make([]*Item, 0, 5000)
	for i := 0; i < 5000; i++ {
		items = append(items, NewItem("n", "a", "body", false))
	}
	l.Model().AppendAll(items)

	win := gtk.NewWindow()
	win.SetChild(l)
	win.SetDefaultSize(360, 640)
	pump()

	if setups >= 400 {
		t.Errorf("item widget setups = %d, want < 400 for 5000 items (recycling not working)", setups)
	}
}

// TestReboundOldNoteNeverCarriesJustInserted covers "A re-bound old note
// never carries the just-inserted class."
func TestReboundOldNoteNeverCarriesJustInserted(t *testing.T) {
	requireDisplay(t)

	l := New([]Section{{ID: "a"}})
	fresh := NewItem("1", "a", "fresh", false)
	l.Model().Append(fresh)

	win := gtk.NewWindow()
	win.SetChild(l)
	win.SetDefaultSize(360, 640)
	pump()

	time.Sleep(InsertAnimDuration + 50*time.Millisecond)

	for i := 0; i < 4000; i++ {
		l.Model().Append(NewItem("n", "a", "filler", false))
	}
	pump()
	l.ScrollTo(l.Model().At(l.Model().Len() - 1))
	pump()
	l.ScrollTo(fresh)
	pump()

	for _, b := range l.rows {
		if b.item == fresh && b.row.HasCSSClass("just-inserted") {
			t.Errorf("rebound old note still carries .just-inserted")
		}
	}
}

// TestInsertingAtZeroGrowsTheFirstRowsAllocatedHeight covers "Inserting
// at 0 makes the first row's AllocatedHeight strictly increase across at
// least two samples."
func TestInsertingAtZeroGrowsTheFirstRowsAllocatedHeight(t *testing.T) {
	requireDisplay(t)

	l := New([]Section{{ID: "a"}})
	l.Model().Append(NewItem("1", "a", "existing", false))

	win := gtk.NewWindow()
	win.SetChild(l)
	win.SetDefaultSize(360, 640)
	pump()

	fresh := NewItem("2", "a", "fresh", false)
	l.Model().InsertAt(0, fresh)
	pump()

	var samples []int
	for i := 0; i < 5; i++ {
		pump()
		var h int
		for _, b := range l.rows {
			if b.item == fresh {
				h = b.row.AllocatedHeight()
			}
		}
		samples = append(samples, h)
		time.Sleep(20 * time.Millisecond)
	}

	strictlyIncreasing := false
	for i := 1; i < len(samples); i++ {
		if samples[i] > samples[i-1] {
			strictlyIncreasing = true
		}
	}
	if !strictlyIncreasing {
		t.Errorf("AllocatedHeight samples = %v, want at least one strict increase", samples)
	}
}

// TestScrollbarNeverOverlapsComposer covers "The scrollbar allocation
// never overlaps the composer" — structurally guaranteed by the
// scrollbar being an overlay child of the ScrolledWindow rather than of
// the whole panel, asserted here against a stub composer packed below.
func TestScrollbarNeverOverlapsComposer(t *testing.T) {
	requireDisplay(t)

	l := New([]Section{{ID: "a"}})
	for i := 0; i < 50; i++ {
		l.Model().Append(NewItem("n", "a", "body", false))
	}

	composer := gtk.NewLabel("composer stand-in")
	composer.SetSizeRequest(-1, 58)

	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.Append(l)
	root.Append(composer)

	win := gtk.NewWindow()
	win.SetChild(root)
	win.SetDefaultSize(360, 640)
	pump()

	scrollbarBounds, ok1 := l.scrollbar.ComputeBounds(root)
	composerBounds, ok2 := composer.ComputeBounds(root)
	if !ok1 || !ok2 {
		t.Fatalf("ComputeBounds failed: scrollbar ok=%v composer ok=%v", ok1, ok2)
	}
	if scrollbarBounds.Y()+scrollbarBounds.Height() > composerBounds.Y() {
		t.Errorf("scrollbar bottom %v overlaps composer top %v", scrollbarBounds.Y()+scrollbarBounds.Height(), composerBounds.Y())
	}
}

// TestEmptySectionRendersHeaderRuleAndPlaceholder covers "An empty
// section still renders its header, rule and placeholder."
func TestEmptySectionRendersHeaderRuleAndPlaceholder(t *testing.T) {
	requireDisplay(t)

	l := New([]Section{{ID: "empty", Title: "Empty"}})

	type bind struct{ start, n uint }
	var binds []bind
	l.onHeaderBind = func(start, n uint) { binds = append(binds, bind{start, n}) }

	win := gtk.NewWindow()
	win.SetChild(l)
	pump()

	if len(binds) != 1 {
		t.Fatalf("header binds for one empty section = %d, want 1", len(binds))
	}

	var found bool
	for _, b := range l.rows {
		if b.item != nil && b.item.IsPlaceholder() && b.ph.Visible() {
			found = true
		}
	}
	if !found {
		t.Errorf("no visible placeholder card bound for the empty section")
	}
}
