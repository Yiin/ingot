package ctrlc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Yiin/ingot/internal/clipboard"
	"github.com/Yiin/ingot/internal/selection"
)

// defaultPollTimeout bounds how long Capture waits for CLIPBOARD to
// change after injecting Ctrl+C. Measured wtype injection is ~19ms; 200ms
// gives the target app generous room to handle the keystroke and write
// its selection without letting a stuck or unresponsive app hang the
// capture indefinitely.
const defaultPollTimeout = 200 * time.Millisecond

// defaultPollInterval is how often Capture rechecks CLIPBOARD while
// waiting for it to change.
const defaultPollInterval = 10 * time.Millisecond

// ErrDisabled is returned by Capture when the Injector was not
// constructed with Enabled: true. The fallback types a keystroke into
// whatever application has focus, invasive enough that library code
// refuses to run it without an explicit, affirmative opt-in — never
// just because a caller forgot to check config first.
var ErrDisabled = errors.New("ctrlc: fallback is not enabled")

// ErrTerminalFocused is returned by Capture when the focused window is a
// terminal, or when focus couldn't be determined at all. Ctrl+C is
// SIGINT in a terminal, not copy: injecting it there would kill the
// user's foreground process, so an unknown focus target is refused
// exactly like a confirmed terminal rather than assumed safe.
var ErrTerminalFocused = errors.New("ctrlc: focused window is a terminal (or unknown); refusing to inject Ctrl+C")

// ErrNoChange is returned by Capture when CLIPBOARD never changed within
// the poll window — the focused app doesn't respond to Ctrl+C by
// writing CLIPBOARD, or nothing was selected. The original clipboard
// snapshot is still restored before this is returned.
var ErrNoChange = errors.New("ctrlc: clipboard did not change after Ctrl+C")

// injectFunc sends a synthetic Ctrl+C keystroke to the compositor.
type injectFunc func(ctx context.Context) error

// focusedClassFunc reports the window class/app-id of the currently
// focused window.
type focusedClassFunc func(ctx context.Context) (string, error)

// Injector performs the opt-in synthetic Ctrl+C capture described in
// the package doc comment.
type Injector struct {
	// Enabled gates every call to Capture; see ErrDisabled.
	Enabled bool

	reader selection.Reader
	writer clipboard.Writer

	inject       injectFunc
	focusedClass focusedClassFunc
	isTerminal   func(class string) bool
	pollInterval time.Duration
	pollTimeout  time.Duration
}

// New returns an Injector backed by wtype for injection and hyprctl for
// focused-window detection — the real implementations. enabled should
// come straight from config.CtrlCFallback.Enabled; there is no default
// here that overrides an explicit "off."
func New(reader selection.Reader, writer clipboard.Writer, enabled bool) *Injector {
	return &Injector{
		Enabled:      enabled,
		reader:       reader,
		writer:       writer,
		inject:       runWtypeCtrlC,
		focusedClass: hyprctlFocusedClass,
		isTerminal:   isTerminalClass,
		pollInterval: defaultPollInterval,
		pollTimeout:  defaultPollTimeout,
	}
}

// Capture runs the full snapshot/inject/poll/restore sequence and
// returns the text the focused application wrote to CLIPBOARD in
// response to the synthetic Ctrl+C. The original CLIPBOARD content is
// always restored before Capture returns, on every path past the
// terminal-focus check — success, ErrNoChange, or a restore failure
// reported alongside whatever text was captured.
func (in *Injector) Capture(ctx context.Context) (string, error) {
	if !in.Enabled {
		return "", ErrDisabled
	}

	class, err := in.focusedClass(ctx)
	if err != nil || in.isTerminal(class) {
		return "", ErrTerminalFocused
	}

	before, err := in.reader.Clipboard(ctx)
	if err != nil && !errors.Is(err, selection.ErrEmpty) {
		return "", fmt.Errorf("ctrlc: snapshot clipboard: %w", err)
	}

	if err := in.inject(ctx); err != nil {
		return "", fmt.Errorf("ctrlc: inject Ctrl+C: %w", err)
	}

	after, changed := in.pollForChange(ctx, before)
	restoreErr := in.writer.SetText(ctx, before)

	if !changed {
		if restoreErr != nil {
			return "", fmt.Errorf("%w (restore also failed: %v)", ErrNoChange, restoreErr)
		}
		return "", ErrNoChange
	}
	if restoreErr != nil {
		return after, fmt.Errorf("ctrlc: restore clipboard: %w", restoreErr)
	}
	return after, nil
}

// pollForChange rechecks CLIPBOARD every pollInterval until it differs
// from before or pollTimeout elapses.
func (in *Injector) pollForChange(ctx context.Context, before string) (after string, changed bool) {
	deadline := time.Now().Add(in.pollTimeout)
	for {
		cur, err := in.reader.Clipboard(ctx)
		if err == nil && cur != before {
			return cur, true
		}
		if !time.Now().Before(deadline) {
			return "", false
		}
		select {
		case <-ctx.Done():
			return "", false
		case <-time.After(in.pollInterval):
		}
	}
}
