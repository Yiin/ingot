package editorwindow

// Manager owns every currently-open editor window, keyed by note ID.
type Manager struct {
	windows map[string]*window
	onSave  func(id, text string)
}

// NewManager returns an empty Manager. onSave runs whenever any open
// window persists a change (its own 400ms debounce, or its close) —
// typically wired to update the store and the panel row.
func NewManager(onSave func(id, text string)) *Manager {
	return &Manager{windows: make(map[string]*window), onSave: onSave}
}

// Open presents the editor window for note, creating it if none is
// already open for note.ID. "Opening a note that already has one
// presents that window instead" — Open is always safe to call, whether
// or not a window for this ID already exists.
func (m *Manager) Open(note Note) {
	if w, ok := m.windows[note.ID]; ok {
		w.present()
		return
	}
	id := note.ID
	w := newWindow(note, m.onSave, func(closedID string) {
		delete(m.windows, closedID)
	})
	m.windows[id] = w
	w.present()
}

// IsOpen reports whether note id already has a live editor window.
func (m *Manager) IsOpen(id string) bool {
	_, ok := m.windows[id]
	return ok
}

// UpdateBody pushes a live change to note id — made elsewhere, e.g. the
// panel row's own inline edit — into its open editor window, if any.
// This is the panel -> editor half of the two-way live sync; the
// editor -> panel half is Manager's own onSave callback. A no-op if id
// has no open window.
func (m *Manager) UpdateBody(id, text string) {
	if w, ok := m.windows[id]; ok {
		w.setBody(text)
	}
}

// Close closes and forgets note id's editor window, flushing any
// pending unsaved change first. A no-op if id has no open window.
func (m *Manager) Close(id string) {
	if w, ok := m.windows[id]; ok {
		w.win.Close()
	}
}

// OpenCount returns how many editor windows are currently open — for
// tests, and any future "Window" menu that lists them.
func (m *Manager) OpenCount() int { return len(m.windows) }
