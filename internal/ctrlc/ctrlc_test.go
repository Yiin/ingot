package ctrlc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/selection"
)

// fakeReader is a selection.Reader whose Clipboard() answers step
// through a scripted sequence, so a test can simulate CLIPBOARD changing
// only after injection "happens."
type fakeReader struct {
	steps []string // "" entries return selection.ErrEmpty instead
	i     int
	calls int
}

func (f *fakeReader) Clipboard(ctx context.Context) (string, error) {
	f.calls++
	if f.i >= len(f.steps) {
		// Repeat the last step once the script runs out, so polling
		// loops don't run past the end of the slice.
		return f.steps[len(f.steps)-1], nil
	}
	v := f.steps[f.i]
	f.i++
	if v == emptySentinel {
		return "", selection.ErrEmpty
	}
	return v, nil
}

func (f *fakeReader) Primary(ctx context.Context) (string, error) {
	return "", errors.New("fakeReader: Primary not used by ctrlc")
}

// emptySentinel marks a script step that should surface selection.ErrEmpty.
const emptySentinel = "\x00empty"

type fakeWriter struct {
	calls []string
	err   error
}

func (f *fakeWriter) SetText(ctx context.Context, text string) error {
	f.calls = append(f.calls, text)
	return f.err
}

func newTestInjector(reader *fakeReader, writer *fakeWriter) *Injector {
	return &Injector{
		Enabled:      true,
		reader:       reader,
		writer:       writer,
		inject:       func(context.Context) error { return nil },
		focusedClass: func(context.Context) (string, error) { return "firefox", nil },
		isTerminal:   isTerminalClass,
		pollInterval: time.Millisecond,
		pollTimeout:  20 * time.Millisecond,
	}
}

func TestCaptureDisabledReturnsErrDisabled(t *testing.T) {
	reader := &fakeReader{}
	writer := &fakeWriter{}
	in := newTestInjector(reader, writer)
	in.Enabled = false
	in.focusedClass = func(context.Context) (string, error) {
		t.Fatal("focusedClass called despite Enabled=false")
		return "", nil
	}

	if _, err := in.Capture(context.Background()); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Capture() err = %v, want ErrDisabled", err)
	}
	if reader.calls != 0 || len(writer.calls) != 0 {
		t.Fatalf("reader/writer touched despite ErrDisabled: reader.calls=%d writer.calls=%v", reader.calls, writer.calls)
	}
}

func TestCaptureRefusesWhenFocusedIsTerminal(t *testing.T) {
	reader := &fakeReader{}
	writer := &fakeWriter{}
	in := newTestInjector(reader, writer)
	in.focusedClass = func(context.Context) (string, error) { return "kitty", nil }
	injected := false
	in.inject = func(context.Context) error { injected = true; return nil }

	if _, err := in.Capture(context.Background()); !errors.Is(err, ErrTerminalFocused) {
		t.Fatalf("Capture() err = %v, want ErrTerminalFocused", err)
	}
	if injected {
		t.Fatalf("Ctrl+C was injected despite a terminal being focused")
	}
	if reader.calls != 0 {
		t.Fatalf("clipboard was read despite a terminal being focused")
	}
}

func TestCaptureRefusesWhenFocusUnknown(t *testing.T) {
	reader := &fakeReader{}
	writer := &fakeWriter{}
	in := newTestInjector(reader, writer)
	in.focusedClass = func(context.Context) (string, error) { return "", errors.New("no active window") }
	injected := false
	in.inject = func(context.Context) error { injected = true; return nil }

	if _, err := in.Capture(context.Background()); !errors.Is(err, ErrTerminalFocused) {
		t.Fatalf("Capture() err = %v, want ErrTerminalFocused when focus is indeterminate", err)
	}
	if injected {
		t.Fatalf("Ctrl+C was injected despite indeterminate focus")
	}
}

func TestCaptureHappyPathRestoresOriginalClipboard(t *testing.T) {
	reader := &fakeReader{steps: []string{"original", "original", "captured selection"}}
	writer := &fakeWriter{}
	in := newTestInjector(reader, writer)

	got, err := in.Capture(context.Background())
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if got != "captured selection" {
		t.Fatalf("Capture() = %q, want %q", got, "captured selection")
	}
	if len(writer.calls) != 1 || writer.calls[0] != "original" {
		t.Fatalf("writer.calls = %v, want a single restore to %q", writer.calls, "original")
	}
}

func TestCaptureNoChangeStillRestoresAndReportsErrNoChange(t *testing.T) {
	reader := &fakeReader{steps: []string{"original"}}
	writer := &fakeWriter{}
	in := newTestInjector(reader, writer)

	_, err := in.Capture(context.Background())
	if !errors.Is(err, ErrNoChange) {
		t.Fatalf("Capture() err = %v, want ErrNoChange", err)
	}
	if len(writer.calls) != 1 || writer.calls[0] != "original" {
		t.Fatalf("writer.calls = %v, want a single restore to %q even on no-op capture", writer.calls, "original")
	}
}

func TestCaptureTreatsInitialEmptyClipboardAsBlank(t *testing.T) {
	reader := &fakeReader{steps: []string{emptySentinel, emptySentinel, "captured selection"}}
	writer := &fakeWriter{}
	in := newTestInjector(reader, writer)

	got, err := in.Capture(context.Background())
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if got != "captured selection" {
		t.Fatalf("Capture() = %q, want %q", got, "captured selection")
	}
	if len(writer.calls) != 1 || writer.calls[0] != "" {
		t.Fatalf("writer.calls = %v, want a single restore to empty", writer.calls)
	}
}

func TestCaptureSurfacesRestoreFailureAlongsideCapturedText(t *testing.T) {
	reader := &fakeReader{steps: []string{"original", "original", "captured selection"}}
	writer := &fakeWriter{err: errors.New("wl-copy: boom")}
	in := newTestInjector(reader, writer)

	got, err := in.Capture(context.Background())
	if err == nil {
		t.Fatalf("Capture() err = nil, want the restore failure surfaced")
	}
	if got != "captured selection" {
		t.Fatalf("Capture() = %q, want the captured text even though restore failed", got)
	}
}
