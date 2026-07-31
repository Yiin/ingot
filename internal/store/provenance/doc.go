// Package provenance persists per-note Created/DoneAt/Source/App/URI
// metadata in an advisory sidecar file, keyed by content hash, so it
// survives note reordering and section moves and degrades to unknown
// after a body edit — honest, since provenance describes a body.
//
// mdfile's Markdown format has no room for this metadata: it round-trips
// only Body and Done. Without a sidecar, every restart would forget when
// a note was created or completed and where a captured note came from.
// The sidecar is advisory only: Load never fails, an absent, empty,
// malformed, or newer-schema file all degrade to no metadata, and Save
// deletes the file outright once nothing remains worth persisting. It
// must never be required for correct operation — deleting it may only
// lose provenance, never a note.
package provenance
