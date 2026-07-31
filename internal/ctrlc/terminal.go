package ctrlc

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
)

// terminalClasses is a best-effort set of window class / app-id values
// that identify a terminal emulator, matched case-insensitively. It is
// necessarily incomplete — there is no exhaustive registry of terminal
// emulators — which is exactly why isTerminalClass treats "unrecognized"
// as "not a terminal" only for values that plainly aren't one, and
// Capture separately treats a focused-window lookup failure as a refusal
// rather than a pass: the failure mode of a false negative here (missing
// an obscure terminal) is a killed foreground process, so the class list
// leans toward including anything terminal-shaped.
var terminalClasses = map[string]bool{
	"foot":                     true,
	"footclient":               true,
	"kitty":                    true,
	"alacritty":                true,
	"wezterm":                  true,
	"org.wezfurlong.wezterm":   true,
	"ghostty":                  true,
	"com.mitchellh.ghostty":    true,
	"gnome-terminal":           true,
	"org.gnome.terminal":       true,
	"org.gnome.console":        true,
	"konsole":                  true,
	"org.kde.konsole":          true,
	"xterm":                    true,
	"uxterm":                   true,
	"urxvt":                    true,
	"rxvt":                     true,
	"st":                       true,
	"terminator":               true,
	"tilix":                    true,
	"xfce4-terminal":           true,
	"io.elementary.terminal":   true,
	"contour":                  true,
	"rio":                      true,
	"blackbox":                 true,
	"com.raggesilver.blackbox": true,
	"terminology":              true,
	"sakura":                   true,
	"termite":                  true,
	"guake":                    true,
	"yakuake":                  true,
	"deepin-terminal":          true,
	"qterminal":                true,
	"lxterminal":               true,
	"mate-terminal":            true,
	"terminal":                 true,
}

// isTerminalClass reports whether class names a known terminal emulator.
// An empty class (focus indeterminate) is treated as a terminal by the
// caller, not by this function — see Capture.
func isTerminalClass(class string) bool {
	return terminalClasses[strings.ToLower(strings.TrimSpace(class))]
}

// activeWindow is the subset of `hyprctl activewindow -j`'s JSON this
// package reads.
type activeWindow struct {
	Class string `json:"class"`
}

// hyprctlFocusedClass shells out to `hyprctl activewindow -j` and
// returns the focused window's class. It returns an error — which
// Capture treats as a refusal, not a pass — for anything short of a
// clean, parseable result: no focused window, hyprctl missing, or
// unparseable output all leave the terminal question genuinely unknown.
func hyprctlFocusedClass(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "hyprctl", "activewindow", "-j")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}

	var win activeWindow
	if err := json.Unmarshal(stdout.Bytes(), &win); err != nil {
		return "", err
	}
	return win.Class, nil
}
