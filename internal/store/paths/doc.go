// Package paths resolves Ingot's XDG directory layout and turns a project
// title into a filesystem-safe slug. Nothing here touches disk directly —
// callers pair it with internal/store/fsx to actually read or write.
package paths
