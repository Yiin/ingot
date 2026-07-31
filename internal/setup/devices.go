package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultSysInputDir = "/sys/class/input"
	defaultDevInputDir = "/dev/input"
)

// keyLeftShiftBit and keyRightShiftBit are the bit positions of
// KEY_LEFTSHIFT (42) and KEY_RIGHTSHIFT (54) in an evdev EV_KEY
// capability mask, per linux/input-event-codes.h. Both fall in the low
// 64 bits, so only the mask's last (least-significant) word needs
// parsing — see isKeyboardCapable.
const (
	keyLeftShiftBit  = 42
	keyRightShiftBit = 54
)

// KeyboardStatus is the result of probing the machine's evdev keyboards
// for whether this process can open them for reading, independent of
// whether Ingot's udev rule is installed.
type KeyboardStatus struct {
	// Detected is how many event nodes sysfs classifies as keyboards, by
	// capability. Sysfs capability files are world-readable, so this
	// count does not depend on the udev rule this package installs.
	Detected int
	// Readable is how many of those Detected nodes this process could
	// actually open for reading.
	Readable int
}

// ProbeKeyboards reports how many keyboard-capable evdev nodes exist and
// how many this process can currently open for reading. An empty sysDir
// or devDir defaults to /sys/class/input or /dev/input; tests pass a
// fixture tree so the probe runs without a real device or root.
func ProbeKeyboards(sysDir, devDir string) (KeyboardStatus, error) {
	if sysDir == "" {
		sysDir = defaultSysInputDir
	}
	if devDir == "" {
		devDir = defaultDevInputDir
	}

	entries, err := os.ReadDir(sysDir)
	if err != nil {
		return KeyboardStatus{}, fmt.Errorf("setup: list %s: %w", sysDir, err)
	}

	var status KeyboardStatus
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "event") {
			continue
		}

		isKeyboard, err := isKeyboardCapable(filepath.Join(sysDir, name))
		if err != nil || !isKeyboard {
			continue
		}
		status.Detected++

		f, err := os.OpenFile(filepath.Join(devDir, name), os.O_RDONLY, 0)
		if err != nil {
			continue
		}
		_ = f.Close()
		status.Readable++
	}
	return status, nil
}

// isKeyboardCapable reports whether the event node whose sysfs directory
// is sysEventDir advertises a Shift key — the capability Ingot's chord
// actually needs, and a reasonable proxy for "this is a keyboard" in
// general.
func isKeyboardCapable(sysEventDir string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(sysEventDir, "device", "capabilities", "key"))
	if err != nil {
		return false, err
	}

	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return false, nil
	}
	// The kernel prints the mask as whitespace-separated 64-bit hex
	// words, most-significant first, trimming leading all-zero words —
	// so the last field is always bits 0-63.
	mask, err := strconv.ParseUint(fields[len(fields)-1], 16, 64)
	if err != nil {
		return false, fmt.Errorf("setup: parse capability mask in %s: %w", sysEventDir, err)
	}
	return mask&(1<<keyLeftShiftBit) != 0 || mask&(1<<keyRightShiftBit) != 0, nil
}
