// Package ctrlc implements the opt-in synthetic Ctrl+C clipboard
// capture: a fallback for apps that fill only the CLIPBOARD selection,
// never PRIMARY, when the user copies (some Electron and GTK apps).
//
// It snapshots CLIPBOARD, injects a synthetic Ctrl+C keystroke into
// whichever application has focus, polls briefly for CLIPBOARD to
// change, then restores the original snapshot — so from the user's own
// clipboard's perspective, nothing happened. Off by default and refused
// outright when the focused window is a terminal, where Ctrl+C is
// SIGINT rather than copy and would kill the user's foreground process.
package ctrlc
