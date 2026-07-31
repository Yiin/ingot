package app

import (
	"log/slog"

	"github.com/Yiin/ingot/internal/store"
)

// wireCompose wires the composer's commit signal to store.AddNote — the
// SourceTyped path, distinct from the chord's SourceCaptured
// AppendToDefault (AppendToDefault always tags SourceCaptured; there is
// no way to make it emit SourceTyped, so composing must go through
// AddNote instead). The composer itself already trims, clears on plain
// Enter, and keeps focus per its own contract — nothing to do here for
// that part of the flow.
func (a *App) wireCompose() {
	a.shell.Composer().OnCommit(safeText("composer-commit", func(text string) {
		sec := a.defaultSectionID()
		if sec == "" {
			slog.Warn("app: compose: no section to add to (active project has none)")
			return
		}
		if _, err := a.store.AddNote(sec, text); err != nil {
			slog.Warn("app: compose: add note", "err", err)
			return
		}
		a.shell.RefreshEmptyState("", 0)
	}))
}

// defaultSectionID returns the active project's last section — the same
// "default capture location" AppendToDefault resolves to — or "" if the
// active project somehow has no sections.
func (a *App) defaultSectionID() store.SectionID {
	proj, err := a.store.Project(a.store.Active())
	if err != nil || len(proj.Sections) == 0 {
		return ""
	}
	return proj.Sections[len(proj.Sections)-1].ID
}
