package editorwindow

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/ui/theme"
)

// Note is the minimal identity Manager and Window need: id is the
// dedup/routing key ("the same note cannot open two editor windows"),
// title is the GtkWindow's own title, and body seeds the buffer.
type Note struct {
	ID    string
	Title string
	Body  string
}

// window is one open editor: a plain 520x420 GtkWindow around a single
// full-bleed GtkTextView. It saves on a 400ms idle debounce after every
// keystroke (via saver) and unconditionally on close — there is no
// OK/Cancel.
type window struct {
	win    *gtk.Window
	buffer *gtk.TextBuffer

	id string

	save *saver
	// suppress is set while setBody is applying an externally-driven
	// change, so that buffer edit is not itself treated as a keystroke
	// to debounce-save.
	suppress bool

	onClosed func(id string)
}

// newWindow builds one editor window for note, unmapped until the
// caller Presents it (see Manager.Open).
func newWindow(note Note, onSave func(id, text string), onClosed func(id string)) *window {
	w := &window{id: note.ID, onClosed: onClosed}
	w.save = newSaver(glibScheduler{}, note.Body, func(text string) {
		if onSave != nil {
			onSave(w.id, text)
		}
	})

	w.buffer = gtk.NewTextBuffer(nil)
	w.buffer.SetText(note.Body)
	w.buffer.PlaceCursor(w.buffer.EndIter())

	view := gtk.NewTextViewWithBuffer(w.buffer)
	view.SetWrapMode(gtk.WrapWordChar)
	view.AddCSSClass("editor-view")
	view.SetTopMargin(theme.EditorPadding)
	view.SetBottomMargin(theme.EditorPadding)
	view.SetLeftMargin(theme.EditorPadding)
	view.SetRightMargin(theme.EditorPadding)

	w.buffer.ConnectChanged(func() {
		if w.suppress {
			return
		}
		w.save.scheduleSave(w.currentText)
	})

	win := gtk.NewWindow()
	win.SetTitle(note.Title)
	win.SetDefaultSize(theme.EditorWidth, theme.EditorHeight)
	// Without this the window keeps the system theme's own background,
	// which on a dark desktop frames the light editor body in near-black.
	// theme's stylesheet hangs the card fill on this class.
	win.AddCSSClass(theme.EditorWindowClass)
	win.SetChild(view)

	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.SetPropagationPhase(gtk.PhaseCapture)
	keyCtrl.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		switch {
		case keyval == gdk.KEY_Escape:
			win.Close()
			return true
		case (keyval == gdk.KEY_w || keyval == gdk.KEY_W) && state&gdk.ControlMask != 0:
			win.Close()
			return true
		default:
			return false
		}
	})
	win.AddController(keyCtrl)

	win.ConnectCloseRequest(func() bool {
		w.save.flush(w.currentText())
		if w.onClosed != nil {
			w.onClosed(w.id)
		}
		return false // false: let the close proceed
	})

	w.win = win
	return w
}

func (w *window) currentText() string {
	return w.buffer.Text(w.buffer.StartIter(), w.buffer.EndIter(), false)
}

func (w *window) present() { w.win.Present() }

// setBody pushes text into the buffer without treating it as a user
// keystroke to debounce-save — the panel-row-edited-elsewhere half of
// the two-way sync (Manager.UpdateBody). A no-op if text already
// matches what this window itself last saved or was last set to, which
// is also what guards against the obvious feedback loop: this window's
// own save firing, the caller persisting it, and the caller then
// calling UpdateBody right back with the same text.
func (w *window) setBody(text string) {
	if !w.save.applyExternal(text) {
		return
	}
	w.suppress = true
	w.buffer.SetText(text)
	w.buffer.PlaceCursor(w.buffer.EndIter())
	w.suppress = false
}
