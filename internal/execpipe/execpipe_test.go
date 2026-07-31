package execpipe

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// scriptOnPath writes an executable shell script at dir/name and returns
// its path.
func scriptOnPath(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func TestRunCapturesStdoutAndStderr(t *testing.T) {
	path := scriptOnPath(t, "cmd.sh", "#!/bin/sh\nprintf 'out'; printf 'err' >&2\n")

	stdout, stderr, err := Run(exec.Command(path), time.Second, 0)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if string(stdout) != "out" {
		t.Errorf("stdout = %q, want %q", stdout, "out")
	}
	if string(stderr) != "err" {
		t.Errorf("stderr = %q, want %q", stderr, "err")
	}
}

func TestRunPropagatesExitError(t *testing.T) {
	path := scriptOnPath(t, "cmd.sh", "#!/bin/sh\nexit 3\n")

	_, _, err := Run(exec.Command(path), time.Second, 0)
	if err == nil {
		t.Fatal("Run() error = nil, want a non-nil exit error")
	}
}

func TestRunCapsStdout(t *testing.T) {
	path := scriptOnPath(t, "cmd.sh", "#!/bin/sh\nhead -c 2000000 /dev/zero\n")

	stdout, _, err := Run(exec.Command(path), time.Second, 1024)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if len(stdout) != 1024 {
		t.Errorf("len(stdout) = %d, want 1024", len(stdout))
	}
}

// TestRunDoesNotHangOnForkedDaemon reproduces the exact bug that hung
// TestSetTextRoundTripsThroughWlPaste: the direct child forks a background
// process that inherits its stdout write end and never closes it, the way
// wl-copy forks a daemon that holds the clipboard selection. Run must
// return once the direct child exits, bounded by grace, instead of
// blocking forever waiting for the forked process to close its copy of
// the pipe.
func TestRunDoesNotHangOnForkedDaemon(t *testing.T) {
	path := scriptOnPath(t, "cmd.sh", "#!/bin/sh\nprintf 'hello'\n(sleep 5 &)\n")

	done := make(chan struct{})
	var stdout []byte
	var err error
	go func() {
		stdout, _, err = Run(exec.Command(path), 200*time.Millisecond, 0)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not return within 3s; it hung on the forked daemon's inherited pipe")
	}

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if string(stdout) != "hello" {
		t.Errorf("stdout = %q, want %q", stdout, "hello")
	}
}
