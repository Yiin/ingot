package selection

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPrimarySuccess(t *testing.T) {
	var gotArgs []string
	r := &WlPasteReader{
		timeout: time.Second,
		run: func(ctx context.Context, args ...string) ([]byte, error) {
			gotArgs = args
			return []byte("captured text"), nil
		},
	}

	got, err := r.Primary(context.Background())
	if err != nil {
		t.Fatalf("Primary() error = %v, want nil", err)
	}
	if got != "captured text" {
		t.Errorf("Primary() = %q, want %q", got, "captured text")
	}
	if len(gotArgs) == 0 || gotArgs[0] != "--primary" {
		t.Errorf("Primary() ran with args %v, want first arg --primary", gotArgs)
	}
}

func TestClipboardSuccess(t *testing.T) {
	var gotArgs []string
	r := &WlPasteReader{
		timeout: time.Second,
		run: func(ctx context.Context, args ...string) ([]byte, error) {
			gotArgs = args
			return []byte("clipboard text"), nil
		},
	}

	got, err := r.Clipboard(context.Background())
	if err != nil {
		t.Fatalf("Clipboard() error = %v, want nil", err)
	}
	if got != "clipboard text" {
		t.Errorf("Clipboard() = %q, want %q", got, "clipboard text")
	}
	for _, a := range gotArgs {
		if a == "--primary" {
			t.Errorf("Clipboard() ran with --primary, args = %v", gotArgs)
		}
	}
}

func TestPrimaryEmpty(t *testing.T) {
	r := &WlPasteReader{
		timeout: time.Second,
		run: func(ctx context.Context, args ...string) ([]byte, error) {
			return nil, errEmptySelection
		},
	}

	_, err := r.Primary(context.Background())
	if !errors.Is(err, ErrEmpty) {
		t.Fatalf("Primary() error = %v, want ErrEmpty", err)
	}
}

func TestPrimaryTimeout(t *testing.T) {
	r := &WlPasteReader{
		timeout: 10 * time.Millisecond,
		run: func(ctx context.Context, args ...string) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	_, err := r.Primary(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Primary() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestPrimaryHonoursCancellation(t *testing.T) {
	r := &WlPasteReader{
		timeout: time.Minute,
		run: func(ctx context.Context, args ...string) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.Primary(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Primary() error = %v, want context.Canceled", err)
	}
}

func TestPrimaryMissingBinary(t *testing.T) {
	notFound := &exec.Error{Name: "wl-paste", Err: exec.ErrNotFound}
	r := &WlPasteReader{
		timeout: time.Second,
		run: func(ctx context.Context, args ...string) ([]byte, error) {
			return nil, notFound
		},
	}

	_, err := r.Primary(context.Background())
	if err == nil {
		t.Fatal("Primary() error = nil, want a wrapped exec.ErrNotFound")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("Primary() error = %v, want it to wrap exec.ErrNotFound", err)
	}
	if errors.Is(err, ErrEmpty) {
		t.Errorf("Primary() error = %v, must not be ErrEmpty", err)
	}
}

// scriptOnPath writes an executable shell script named "wl-paste" into a
// fresh directory, points PATH at it, and returns the directory. This
// exercises the real runWlPaste exec plumbing end to end without needing
// an actual wl-paste binary or a live Wayland session.
func scriptOnPath(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "wl-paste")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write fake wl-paste: %v", err)
	}
	// The fake wl-paste must resolve first, but scripts still need real
	// coreutils (head, cat, ...) on the rest of PATH to do their job.
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunWlPasteSuccess(t *testing.T) {
	scriptOnPath(t, `printf 'hello there'`)

	out, err := runWlPaste(context.Background(), "--no-newline")
	if err != nil {
		t.Fatalf("runWlPaste() error = %v, want nil", err)
	}
	if string(out) != "hello there" {
		t.Errorf("runWlPaste() = %q, want %q", out, "hello there")
	}
}

func TestRunWlPasteEmpty(t *testing.T) {
	scriptOnPath(t, `echo "Nothing is copied" >&2; exit 1`)

	_, err := runWlPaste(context.Background(), "--no-newline")
	if !errors.Is(err, errEmptySelection) {
		t.Fatalf("runWlPaste() error = %v, want errEmptySelection", err)
	}
}

func TestRunWlPasteOtherExitOneIsNotEmpty(t *testing.T) {
	scriptOnPath(t, `echo "some unrelated failure" >&2; exit 1`)

	_, err := runWlPaste(context.Background(), "--no-newline")
	if err == nil {
		t.Fatal("runWlPaste() error = nil, want a generic exit error")
	}
	if errors.Is(err, errEmptySelection) {
		t.Errorf("runWlPaste() error = %v, must not be classified as empty", err)
	}
}

func TestRunWlPasteMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := runWlPaste(context.Background(), "--no-newline")
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("runWlPaste() error = %v, want it to wrap exec.ErrNotFound", err)
	}
}

func TestRunWlPasteCapsOutput(t *testing.T) {
	scriptOnPath(t, `head -c 2000000 /dev/zero`)

	out, err := runWlPaste(context.Background(), "--no-newline")
	if err != nil {
		t.Fatalf("runWlPaste() error = %v, want nil", err)
	}
	if len(out) != maxReadBytes {
		t.Errorf("runWlPaste() returned %d bytes, want capped at %d", len(out), maxReadBytes)
	}
}

func TestNewReaderDefaults(t *testing.T) {
	r := NewReader()
	if r.run == nil {
		t.Fatal("NewReader().run is nil")
	}
	if r.timeout != defaultTimeout {
		t.Errorf("NewReader().timeout = %v, want %v", r.timeout, defaultTimeout)
	}
}
