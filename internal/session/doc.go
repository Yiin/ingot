// Package session tracks the desktop session lock state over logind's
// system D-Bus interface, and gates internal/input so that no evdev reads
// happen while the session is locked. Ingot holds every keyboard device
// open for the double-Shift chord, so continuing to read while locked
// would be a real keylogging exposure, not just wasted work.
package session
