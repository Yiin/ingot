package ctrlc

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRunWtypeCtrlCSuccess(t *testing.T) {
	writeFakeBinary(t, "wtype", "", 0)
	if err := runWtypeCtrlC(context.Background()); err != nil {
		t.Fatalf("runWtypeCtrlC: %v", err)
	}
}

func TestRunWtypeCtrlCMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := runWtypeCtrlC(context.Background()); err == nil {
		t.Fatalf("runWtypeCtrlC with no wtype on PATH: got nil error")
	}
}

func TestRunWtypeCtrlCFailureSurfacesStderr(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'no seat available' >&2\nexit 1\n"
	path := dir + "/wtype"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("PATH", dir)

	err := runWtypeCtrlC(context.Background())
	if err == nil {
		t.Fatalf("runWtypeCtrlC: got nil error, want the script's failure")
	}
	if !strings.Contains(err.Error(), "no seat available") {
		t.Errorf("err = %v, want it to contain the script's stderr", err)
	}
}
