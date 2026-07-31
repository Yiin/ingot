package app

import (
	"path/filepath"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/ui/notelist"
)

// storeAdapter translates store.Event into notelist.Model mutations for
// the active project. internal/ui/notelist has no internal/store
// dependency of its own (see its package doc) — this is the glue this
// integration child adds. It tracks the active project's sections in
// store order alongside the model's own order, so NotesSpliced's
// position-and-count shape can be turned into the right InsertAt/
// RemoveAt calls without the model exposing store ids itself.
//
// Every method must run on the GTK thread: Seed is called once from
// startup before the store is ever subscribed to, and OnEvent is
// subscribed via store.Subscribe, which (per fsstore's Options.Post
// being wired to gtkapp.App.Post) only ever fires there.
type storeAdapter struct {
	store  store.Store
	model  *notelist.Model
	notify func(msg string)

	activeIdx int
	items     map[string][]*notelist.Item // section id -> its notes, store order
	byID      map[string]*notelist.Item
}

func newStoreAdapter(st store.Store, model *notelist.Model, notify func(msg string)) *storeAdapter {
	if notify == nil {
		notify = func(string) {}
	}
	return &storeAdapter{
		store:  st,
		model:  model,
		notify: notify,
		items:  make(map[string][]*notelist.Item),
		byID:   make(map[string]*notelist.Item),
	}
}

// Seed populates the model from the store's current active project. Call
// once, before Subscribe, so the first live event never double-applies
// what Seed already loaded.
func (a *storeAdapter) Seed() {
	a.rebuild()
}

// OnEvent applies one store.Event to the model.
func (a *storeAdapter) OnEvent(ev store.Event) {
	switch e := ev.(type) {
	case store.NotesSpliced:
		a.onSpliced(e)
	case store.NoteUpdated:
		a.onUpdated(e)
	case store.ActiveProjectChanged, store.ProjectListChanged, store.SectionsChanged, store.ProjectReloaded:
		a.rebuild()
	case store.ConflictResolved:
		a.rebuild()
		a.notify("Reloaded from disk — your version saved to " + filepath.Base(e.SavedTo))
	case store.SaveFailed:
		a.notify("Couldn't save — will keep retrying")
	case store.ProjectReadOnly:
		a.notify("This project can't be saved (file has unexpected content)")
	}
}

// activeProjectIndex returns the active project's position in
// store.Projects() — the same coordinate space NotesSpliced.Project and
// NoteUpdated.Project index into — or -1 if there is somehow no active
// project.
func (a *storeAdapter) activeProjectIndex() int {
	active := a.store.Active()
	for i, p := range a.store.Projects() {
		if p.ID == active {
			return i
		}
	}
	return -1
}

// rebuild reloads the active project wholesale: declared sections, every
// note, and the adapter's own mirror. Used for the initial Seed and
// every event that can change section shape or swap the active project.
func (a *storeAdapter) rebuild() {
	a.activeIdx = a.activeProjectIndex()
	a.items = make(map[string][]*notelist.Item)
	a.byID = make(map[string]*notelist.Item)

	if a.activeIdx < 0 {
		a.model.SetSections(nil)
		a.model.Reset(nil)
		return
	}

	proj, err := a.store.Project(a.store.Active())
	if err != nil {
		return
	}

	a.model.SetSections(storeSectionsToNotelist(proj.Sections))

	var all []*notelist.Item
	for _, s := range proj.Sections {
		secID := string(s.ID)
		secItems := make([]*notelist.Item, 0, len(s.Notes))
		for _, n := range s.Notes {
			it := notelist.NewItem(string(n.ID), secID, n.Body, n.Done)
			secItems = append(secItems, it)
			a.byID[string(n.ID)] = it
			all = append(all, it)
		}
		a.items[secID] = secItems
	}
	a.model.Reset(all)
}

// onSpliced applies one NotesSpliced event: a contiguous
// insertion/removal within one section of the active project.
func (a *storeAdapter) onSpliced(e store.NotesSpliced) {
	if e.Project != a.activeIdx {
		return
	}
	proj, err := a.store.Project(a.store.Active())
	if err != nil || e.Section >= len(proj.Sections) {
		a.rebuild()
		return
	}
	sec := proj.Sections[e.Section]
	secID := string(sec.ID)
	list := a.items[secID]

	if e.Removed > 0 {
		if e.Index+e.Removed > len(list) {
			a.rebuild()
			return
		}
		for i := e.Index; i < e.Index+e.Removed; i++ {
			it := list[i]
			if pos := a.model.IndexOf(it); pos >= 0 {
				a.model.RemoveAt(pos)
			}
			delete(a.byID, it.ID)
		}
		list = append(append([]*notelist.Item(nil), list[:e.Index]...), list[e.Index+e.Removed:]...)
	}

	if e.Added > 0 {
		if e.Index+e.Added > len(sec.Notes) {
			a.rebuild()
			return
		}
		fresh := make([]*notelist.Item, e.Added)
		for k := 0; k < e.Added; k++ {
			n := sec.Notes[e.Index+k]
			it := notelist.NewItem(string(n.ID), secID, n.Body, n.Done)
			fresh[k] = it
			a.byID[string(n.ID)] = it
		}

		switch {
		case e.Index < len(list):
			base := a.model.IndexOf(list[e.Index])
			for k, it := range fresh {
				a.model.InsertAt(base+k, it)
			}
		case len(list) > 0:
			base := a.model.IndexOf(list[len(list)-1])
			for k, it := range fresh {
				a.model.InsertAt(base+1+k, it)
			}
		default:
			for _, it := range fresh {
				a.model.Append(it)
			}
		}

		out := make([]*notelist.Item, 0, len(list)+len(fresh))
		out = append(out, list[:e.Index]...)
		out = append(out, fresh...)
		out = append(out, list[e.Index:]...)
		list = out
	}

	a.items[secID] = list
}

// onUpdated applies one NoteUpdated event: an in-place body/done change
// that doesn't move the note.
func (a *storeAdapter) onUpdated(e store.NoteUpdated) {
	if e.Project != a.activeIdx {
		return
	}
	it, ok := a.byID[string(e.ID)]
	if !ok {
		return
	}
	n, err := a.store.Note(e.ID)
	if err != nil {
		return
	}
	if it.Body == n.Body && it.Done == n.Done {
		return
	}
	it.Body = n.Body
	it.Done = n.Done
	a.model.Refresh(it)
}

// itemForNote returns the currently bound *notelist.Item for id, or nil.
// Used by the capture flow to flash a duplicate's existing row.
func (a *storeAdapter) itemForNote(id store.NoteID) *notelist.Item {
	return a.byID[string(id)]
}

func storeSectionsToNotelist(sections []store.Section) []notelist.Section {
	out := make([]notelist.Section, len(sections))
	for i, s := range sections {
		out[i] = notelist.Section{ID: string(s.ID), Title: s.Title}
	}
	return out
}
