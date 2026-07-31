package selection

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// ErrEmpty means wl-paste reported that nothing is currently
// selected/copied. Callers should treat it as a silent no-op, not as a
// failure worth surfacing to the user.
var ErrEmpty = errors.New("selection: nothing is copied")

// errEmptySelection is the sentinel runWlPaste wraps when wl-paste exits 1
// with "Nothing is copied" on stderr — the documented way it reports an
// empty selection, distinct from every other failure mode.
var errEmptySelection = errors.New("nothing is copied")

// maxReadBytes caps how much text a single read returns. A selection this
// large is not something a quick-capture note needs, and an unbounded read
// would let a misbehaving selection owner exhaust memory.
const maxReadBytes = 1 << 20 // 1 MiB

// defaultTimeout bounds how long a read may block. wl-paste normally
// returns in single-digit milliseconds; anything longer means the
// compositor or the selection owner is stuck.
const defaultTimeout = 2 * time.Second

// Reader reads the Wayland PRIMARY selection and CLIPBOARD.
type Reader interface {
	Primary(ctx context.Context) (string, error)
	Clipboard(ctx context.Context) (string, error)
}

// runner abstracts the wl-paste invocation so tests can drive success,
// empty-selection, timeout, and missing-binary paths without a real
// Wayland session or a real wl-paste binary.
type runner func(ctx context.Context, args ...string) ([]byte, error)

// WlPasteReader is the real, wl-paste-backed Reader.
type WlPasteReader struct {
	run     runner
	timeout time.Duration
}

// NewReader returns a Reader backed by the wl-paste binary on PATH.
func NewReader() *WlPasteReader {
	return &WlPasteReader{run: runWlPaste, timeout: defaultTimeout}
}

// Primary reads the PRIMARY selection.
func (r *WlPasteReader) Primary(ctx context.Context) (string, error) {
	return r.read(ctx, "--primary", "--no-newline")
}

// Clipboard reads CLIPBOARD.
func (r *WlPasteReader) Clipboard(ctx context.Context) (string, error) {
	return r.read(ctx, "--no-newline")
}

func (r *WlPasteReader) read(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	out, err := r.run(ctx, args...)
	if err != nil {
		if errors.Is(err, errEmptySelection) {
			return "", ErrEmpty
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("selection: %w", ctx.Err())
		}
		return "", fmt.Errorf("selection: wl-paste: %w", err)
	}
	return string(out), nil
}

// limitWriter caps the bytes it keeps at limit, silently discarding
// anything past it, while still reporting every byte as written so the
// wl-paste process is never blocked or failed by the cap.
type limitWriter struct {
	buf   bytes.Buffer
	limit int
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if room := w.limit - w.buf.Len(); room > 0 {
		if room > len(p) {
			room = len(p)
		}
		w.buf.Write(p[:room])
	}
	return len(p), nil
}

func runWlPaste(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "wl-paste", args...)
	stdout := &limitWriter{limit: maxReadBytes}
	var stderr bytes.Buffer
	cmd.Stdout = stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return stdout.buf.Bytes(), nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && bytes.Contains(stderr.Bytes(), []byte("Nothing is copied")) {
		return nil, errEmptySelection
	}
	return stdout.buf.Bytes(), err
}
