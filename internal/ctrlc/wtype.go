package ctrlc

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

// runWtypeCtrlC injects a Ctrl+C keystroke via wtype's virtual-keyboard
// protocol (zwp_virtual_keyboard_manager_v1). wtype is used rather than
// ydotool for two decisive, measured reasons: wtype needs no daemon, and
// its injection is invisible to evdev, whereas ydotool's injection
// arrives from its own virtual device and would feed Ingot's evdev-based
// chord detector a loop. "-M ctrl" presses the modifier, "-k c" presses
// and releases the C key, "-m ctrl" releases the modifier.
func runWtypeCtrlC(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "wtype", "-M", "ctrl", "-k", "c", "-m", "ctrl")
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
