//go:build integration

// These tests need a real GDK display connection (gtk.Init +
// gdk.DisplayGetDefault), so they are gated behind the integration tag —
// same convention as `make test-integration` — and need to run inside the
// headless sway harness from copper-l2z.31 (WLR_BACKENDS=headless,
// GSK_RENDERER=cairo). Do not run this file against a live desktop
// session; nothing here is ever shown, but it is still a real GTK client
// on whatever WAYLAND_DISPLAY points at.
package theme_test

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/theme"
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

// recordingHandler captures every slog record so the test can inspect the
// GTK CSS parser warnings gotk4's slog bridge (glib.LogSetWriterFunc)
// routes through the default logger. See theme.go's comment on why the
// tests assert on this instead of gtk.CSSProvider.ConnectParsingError.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordingHandler) messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	msgs := make([]string, len(h.records))
	for i, r := range h.records {
		msgs[i] = r.Message
	}
	return msgs
}

// TestEmbeddedCSSParsesCleanly loads the embedded stylesheet under a real
// display and asserts GTK never logged a "Theme parser error" line for it.
func TestEmbeddedCSSParsesCleanly(t *testing.T) {
	display := requireDisplay(t)

	handler := &recordingHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(prev) })

	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	for _, msg := range handler.messages() {
		if strings.Contains(msg, "Theme parser error") {
			t.Errorf("CSS provider logged a parser error: %s", msg)
		}
	}
}

// TestBodyLabelLineHeight renders a body-styled GtkLabel and checks its
// single line measures theme.LineBody (18) px, per --line-body / --font-body
// in style.css.
func TestBodyLabelLineHeight(t *testing.T) {
	display := requireDisplay(t)
	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	label := gtk.NewLabel("One line of body text")
	label.AddCSSClass("ingot-panel")

	// Never shown: giving the label a GtkRoot is enough for GTK to compute
	// its style and Pango layout without mapping any surface on screen.
	win := gtk.NewWindow()
	win.SetChild(label)

	if lines := label.Layout().LineCount(); lines != 1 {
		t.Fatalf("label rendered %d lines, want 1", lines)
	}
	if _, height := label.Layout().PixelSize(); height != theme.LineBody {
		t.Errorf("body label line height = %dpx, want %dpx", height, theme.LineBody)
	}
}

// TestSelectedRowLabelColorMatchesInk guards copper-l2z.67: Adwaita's
// default row:selected chrome sets a light foreground colour that
// inherits down into a child GtkLabel, which would otherwise render
// near-white on .note-card.selected's light-blue background. A
// GtkListBoxRow's CSS node is named "row" like a GtkListView's recycled
// item, so it reproduces the same cascade .ingot-notelist > row:selected
// resets against.
func TestSelectedRowLabelColorMatchesInk(t *testing.T) {
	display := requireDisplay(t)
	if err := theme.Load(display); err != nil {
		t.Fatalf("theme.Load: %v", err)
	}

	list := gtk.NewListBox()
	list.AddCSSClass("ingot-notelist")
	list.SetSelectionMode(gtk.SelectionSingle)

	label := gtk.NewLabel("Note body")
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("note-card")
	card.AddCSSClass("selected")
	card.Append(label)

	list.Append(card)

	win := gtk.NewWindow()
	win.SetChild(list)

	row := list.RowAtIndex(0)
	if row == nil {
		t.Fatal("ListBox has no row at index 0")
	}
	list.SelectRow(row)
	if !row.IsSelected() {
		t.Fatal("row did not report itself selected after SelectRow")
	}

	// gdk.RGBA wraps a cgo-allocated pointer (gextras.StructNative), so a
	// bare `var want gdk.RGBA` leaves that pointer nil and Parse segfaults
	// dereferencing it — construct through gdk.NewRGBA to get a real
	// allocation first.
	want := gdk.NewRGBA(0, 0, 0, 1)
	if !want.Parse(theme.Ink) {
		t.Fatalf("gdk.RGBA.Parse(%q) failed", theme.Ink)
	}

	got := label.StyleContext().Color()
	// Not gdk.RGBA.Equal: gotk4 v0.4.0's binding dereferences the native
	// pointer one level too many (StructNative already returns the
	// *C.GdkRGBA, but Equal casts it to *gconstpointer and dereferences
	// again), reading the struct's own red/green float bytes as if they
	// were a pointer and segfaulting inside gdk_rgba_equal. The plain
	// field accessors below go through the struct directly and don't
	// have this bug.
	equal := got.Red() == want.Red() && got.Green() == want.Green() &&
		got.Blue() == want.Blue() && got.Alpha() == want.Alpha()
	if !equal {
		t.Errorf("selected note label color = %s, want %s (--ink)", got.String(), want.String())
	}
}
