// Package panel assembles the note list, search bar and composer into the
// panel's own widget tree — the opaque 360dp rounded card the whole app
// lives in — and covers every empty and edge state the demo video never
// shows: first run, an empty section, a search with no matches, the two
// capture-outcome toasts, and the unfocused visual state.
//
// This package returns a widget tree and knows nothing about the Wayland
// surface: sizing the surface itself (the 72% work-area height cap, the
// right-edge anchor) is internal/layershell's job (copper-l2z.19). Shell
// only makes sure its own tree behaves when an external caller constrains
// its height — the list scrolls, the search field and composer stay
// pinned — which falls out of ordinary GTK box packing once the list is
// the only VExpand child.
//
// Shell also owns no store and runs no search of its own: it has no
// internal/store dependency at all. Creating notes, mutating them, and
// deciding what "duplicate" or "empty selection" means during a capture
// are copper-l2z.30's job; Shell only renders the outcome once told —
// see RefreshEmptyState, NotifyEmptySelection and NotifyDuplicate.
package panel
