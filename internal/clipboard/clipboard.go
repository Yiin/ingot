package clipboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// defaultTimeout bounds how long SetText may block on wl-copy starting and
// handing its payload to the forked background server.
const defaultTimeout = 2 * time.Second

// Writer sets the Wayland CLIPBOARD selection.
type Writer interface {
	SetText(ctx context.Context, text string) error
}

// Fallback sets the clipboard by some means other than wl-copy. It is only
// invoked when wl-copy is not on PATH.
type Fallback func(ctx context.Context, text string) error

// runner abstracts the wl-copy invocation so tests can drive success and
// failure paths without a real Wayland session or a real wl-copy binary.
type runner func(ctx context.Context, text string) error

// WlCopyWriter is the real, wl-copy-backed Writer.
type WlCopyWriter struct {
	run      runner
	lookPath func(string) (string, error)
	fallback Fallback
	timeout  time.Duration
}

// NewWriter returns a Writer backed by the wl-copy binary on PATH. fallback
// may be nil, in which case SetText returns an error instead of degrading
// when wl-copy is missing.
func NewWriter(fallback Fallback) *WlCopyWriter {
	return &WlCopyWriter{
		run:      runWlCopy,
		lookPath: exec.LookPath,
		fallback: fallback,
		timeout:  defaultTimeout,
	}
}

// SetText sets CLIPBOARD to text via wl-copy, or via the configured
// Fallback when wl-copy is not on PATH.
func (w *WlCopyWriter) SetText(ctx context.Context, text string) error {
	if _, err := w.lookPath("wl-copy"); err != nil {
		if w.fallback != nil {
			return w.fallback(ctx, text)
		}
		return fmt.Errorf("clipboard: wl-copy not found and no fallback configured: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	if err := w.run(ctx, text); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("clipboard: %w", ctx.Err())
		}
		return fmt.Errorf("clipboard: wl-copy: %w", err)
	}
	return nil
}

func runWlCopy(ctx context.Context, text string) error {
	cmd := exec.CommandContext(ctx, "wl-copy")
	cmd.Stdin = strings.NewReader(text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return errors.New(msg)
		}
		return err
	}
	return nil
}
