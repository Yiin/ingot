// Package fsx is the store's filesystem seam: a nine-method interface that
// lets every store package be exercised against an in-memory filesystem,
// including fault injection, with no real disk and no GTK runtime.
//
// It also carries the two crash-safety primitives every writer in the
// store builds on: AtomicWrite (temp file + rename, so the live file is
// never partially written) and RotateBackup (hardlink-based backup
// rotation with a byte-copy fallback across filesystems).
package fsx
