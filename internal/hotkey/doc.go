// Package hotkey detects the double-tap Shift chord from a stream of key
// events. Detector is a pure state machine: it takes all timing from
// input.Event.At rather than the clock, so it needs no I/O and no sleeping
// in tests. The double-tap window is configurable (see internal/config);
// the maximum hold time for a single clean tap is not.
package hotkey
