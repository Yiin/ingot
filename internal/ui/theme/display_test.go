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
