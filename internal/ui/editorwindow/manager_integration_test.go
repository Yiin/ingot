//go:build integration

// These tests need a real GDK display connection (gtk.Init + a mapped
// GtkWindow, since the debounced save relies on GLib's default main
// context actually running timeouts), so they are gated behind the
// integration tag and need copper-l2z.31's headless sway harness
// (WLR_BACKENDS=headless, GSK_RENDERER=cairo) to actually run — same
// convention as internal/ui/composer's own integration tests. This
// worktree has no display at all: these tests only need to compile here
// (go vet -tags integration), never execute.
package editorwindow

import (
	"sync"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/theme"
)

var gtkInitOnce sync.Once

func pump() {
	ctx := glib.MainContextDefault()
	for ctx.Pending() {
		ctx.Iteration(false)
	}
}

func pumpFor(d time.Duration) {
	deadline := time.Now().Add(d)
	ctx := glib.MainContextDefault()
	for time.Now().Before(deadline) {
		for ctx.Pending() {
			ctx.Iteration(false)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func newTestManager(t *testing.T) (*Manager, *[]string) {
	t.Helper()
	gtkInitOnce.Do(gtk.Init)

	var saved []string
	m := NewManager(func(id, text string) {
		saved = append(saved, id+":"+text)
	})
	return m, &saved
}

// TestOpenTwiceReusesWindow covers "the same note cannot open two
// editor windows": Open called twice for the same ID must not create a
// second one.
func TestOpenTwiceReusesWindow(t *testing.T) {
	m, _ := newTestManager(t)

	m.Open(Note{ID: "n1", Title: "Note", Body: "hello"})
	m.Open(Note{ID: "n1", Title: "Note", Body: "hello"})

	if got := m.OpenCount(); got != 1 {
		t.Errorf("OpenCount() = %d, want 1", got)
	}
	if !m.IsOpen("n1") {
		t.Error("IsOpen(\"n1\") = false, want true")
	}
}

// TestKeystrokeSavesWithinDebounce covers "a keystroke in the editor
// updates the panel row within one debounce interval."
func TestKeystrokeSavesWithinDebounce(t *testing.T) {
	m, saved := newTestManager(t)
	m.Open(Note{ID: "n1", Title: "Note", Body: "hello"})

	w := m.windows["n1"]
	w.buffer.SetText("hello world")
	pumpFor(theme.EditorSaveDebounceMs*time.Millisecond + 100*time.Millisecond)

	if len(*saved) == 0 {
		t.Fatal("onSave never fired within one debounce interval")
	}
}

// TestCloseWithoutWaitPersists covers "closing without an explicit save
// persists the text" — closing immediately after a keystroke, well
// before the debounce would have fired on its own.
func TestCloseWithoutWaitPersists(t *testing.T) {
	m, saved := newTestManager(t)
	m.Open(Note{ID: "n1", Title: "Note", Body: "hello"})

	w := m.windows["n1"]
	w.buffer.SetText("hello world")
	m.Close("n1")
	pump()

	if len(*saved) == 0 {
		t.Fatal("closing did not flush the pending save")
	}
	if m.IsOpen("n1") {
		t.Error("IsOpen(\"n1\") = true after Close, want false")
	}
}

// TestUpdateBodyPushesIntoOpenWindow covers the panel -> editor half of
// "two-way live sync."
func TestUpdateBodyPushesIntoOpenWindow(t *testing.T) {
	m, _ := newTestManager(t)
	m.Open(Note{ID: "n1", Title: "Note", Body: "hello"})

	m.UpdateBody("n1", "edited elsewhere")
	pump()

	w := m.windows["n1"]
	got := w.buffer.Text(w.buffer.StartIter(), w.buffer.EndIter(), false)
	if got != "edited elsewhere" {
		t.Errorf("buffer text = %q, want %q", got, "edited elsewhere")
	}
}
