package clipboard

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSetTextSuccess(t *testing.T) {
	var gotText string
	w := &WlCopyWriter{
		timeout:  time.Second,
		lookPath: func(string) (string, error) { return "/usr/bin/wl-copy", nil },
		run: func(ctx context.Context, text string) error {
			gotText = text
			return nil
		},
	}

	if err := w.SetText(context.Background(), "1. first\n2. second"); err != nil {
		t.Fatalf("SetText() error = %v, want nil", err)
	}
	if gotText != "1. first\n2. second" {
		t.Errorf("SetText() ran wl-copy with %q", gotText)
	}
}

func TestSetTextRunFailure(t *testing.T) {
	wantErr := errors.New("wl-copy: connection refused")
	w := &WlCopyWriter{
		timeout:  time.Second,
		lookPath: func(string) (string, error) { return "/usr/bin/wl-copy", nil },
		run: func(ctx context.Context, text string) error {
			return wantErr
		},
	}

	err := w.SetText(context.Background(), "text")
	if !errors.Is(err, wantErr) {
		t.Fatalf("SetText() error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestSetTextTimeout(t *testing.T) {
	w := &WlCopyWriter{
		timeout:  10 * time.Millisecond,
		lookPath: func(string) (string, error) { return "/usr/bin/wl-copy", nil },
		run: func(ctx context.Context, text string) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	err := w.SetText(context.Background(), "text")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("SetText() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestSetTextFallsBackWhenWlCopyMissing(t *testing.T) {
	var fallbackText string
	fallbackCalled := false
	w := &WlCopyWriter{
		timeout:  time.Second,
		lookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		run: func(ctx context.Context, text string) error {
			t.Fatal("run must not be called when wl-copy is missing")
			return nil
		},
		fallback: func(ctx context.Context, text string) error {
			fallbackCalled = true
			fallbackText = text
			return nil
		},
	}

	if err := w.SetText(context.Background(), "fallback text"); err != nil {
		t.Fatalf("SetText() error = %v, want nil", err)
	}
	if !fallbackCalled {
		t.Fatal("SetText() did not call the fallback")
	}
	if fallbackText != "fallback text" {
		t.Errorf("fallback got %q, want %q", fallbackText, "fallback text")
	}
}

func TestSetTextErrorsWithoutFallbackWhenWlCopyMissing(t *testing.T) {
	w := &WlCopyWriter{
		timeout:  time.Second,
		lookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		run: func(ctx context.Context, text string) error {
			t.Fatal("run must not be called when wl-copy is missing")
			return nil
		},
		fallback: nil,
	}

	err := w.SetText(context.Background(), "text")
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("SetText() error = %v, want it to wrap exec.ErrNotFound", err)
	}
}

func TestNewWriterDefaults(t *testing.T) {
	w := NewWriter(nil)
	if w.run == nil {
		t.Fatal("NewWriter().run is nil")
	}
	if w.lookPath == nil {
		t.Fatal("NewWriter().lookPath is nil")
	}
	if w.timeout != defaultTimeout {
		t.Errorf("NewWriter().timeout = %v, want %v", w.timeout, defaultTimeout)
	}
	if w.fallback != nil {
		t.Error("NewWriter(nil).fallback must be nil")
	}
}

// scriptOnPath writes an executable shell script named "wl-copy" into a
// fresh directory and points PATH at it, exercising the real runWlCopy
// exec plumbing end to end without a real wl-copy binary or Wayland
// session.
func scriptOnPath(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "wl-copy")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("write fake wl-copy: %v", err)
	}
	// The fake wl-copy must resolve first, but scripts still need real
	// coreutils (cat, ...) on the rest of PATH to do their job.
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunWlCopySuccess(t *testing.T) {
	out := filepath.Join(t.TempDir(), "captured")
	scriptOnPath(t, `cat > `+out)

	if err := runWlCopy(context.Background(), "note one\nnote two"); err != nil {
		t.Fatalf("runWlCopy() error = %v, want nil", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read captured stdin: %v", err)
	}
	if string(got) != "note one\nnote two" {
		t.Errorf("wl-copy received %q, want %q", got, "note one\nnote two")
	}
}

func TestRunWlCopyFailure(t *testing.T) {
	scriptOnPath(t, `echo "no clipboard manager" >&2; exit 1`)

	err := runWlCopy(context.Background(), "text")
	if err == nil {
		t.Fatal("runWlCopy() error = nil, want an error")
	}
}

func TestRunWlCopyMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := runWlCopy(context.Background(), "text")
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("runWlCopy() error = %v, want it to wrap exec.ErrNotFound", err)
	}
}
