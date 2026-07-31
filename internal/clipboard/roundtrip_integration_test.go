//go:build integration

// This test needs a live Wayland session (or the headless harness from
// copper-l2z.31) with real wl-copy/wl-paste binaries on PATH. It is not
// run by the normal `make test`; it only runs under `make test-integration`
// / `go test -tags integration`.
package clipboard_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/Yiin/ingot/internal/clipboard"
	"github.com/Yiin/ingot/internal/selection"
)

// TestSetTextRoundTripsThroughWlPaste verifies the acceptance criterion
// that text set via clipboard.Writer, from a process with no keyboard
// focus (this test binary is not a layer-shell surface at all), is
// readable back via wl-paste and survives this process continuing to run —
// i.e. it lands on wl-copy's forked background server, not held in-process.
func TestSetTextRoundTripsThroughWlPaste(t *testing.T) {
	if _, err := exec.LookPath("wl-copy"); err != nil {
		t.Skip("wl-copy not on PATH")
	}
	if _, err := exec.LookPath("wl-paste"); err != nil {
		t.Skip("wl-paste not on PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	w := clipboard.NewWriter(nil)
	want := "1. round trip note\n2. second note"
	if err := w.SetText(ctx, want); err != nil {
		t.Fatalf("SetText() error = %v", err)
	}

	r := selection.NewReader()
	got, err := r.Clipboard(ctx)
	if err != nil {
		t.Fatalf("Clipboard() error = %v", err)
	}
	if got != want {
		t.Errorf("Clipboard() = %q, want %q", got, want)
	}
}
