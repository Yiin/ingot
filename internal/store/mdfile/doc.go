// Package mdfile implements the Markdown grammar Ingot stores projects
// in: Parse turns a project file's bytes into a store.Project, Format
// turns a store.Project back into canonical bytes. It uses a
// hand-written line scanner, not goldmark — goldmark belongs on the
// display path in internal/ui/mdpango, not here, so this package keeps
// exact round-trip control with no dependency on a rendering library's
// block rules.
package mdfile
