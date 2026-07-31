package session

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	loginBusName        = "org.freedesktop.login1"
	loginObjectPath     = dbus.ObjectPath("/org/freedesktop/login1")
	managerInterface    = "org.freedesktop.login1.Manager"
	sessionInterface    = "org.freedesktop.login1.Session"
	propertiesInterface = "org.freedesktop.DBus.Properties"

	lockSignalName              = sessionInterface + ".Lock"
	unlockSignalName            = sessionInterface + ".Unlock"
	propertiesChangedSignalName = propertiesInterface + ".PropertiesChanged"
)

// LogindLockState watches the caller's logind session for its Lock and
// Unlock signals over the system D-Bus. logind is the substrate every
// desktop session backend already reports through — GNOME, KDE, and every
// wlroots compositor via logind's session tracking — so it is the
// portable primary source here, not a fallback:
// ext_session_lock_manager_v1 is Wayland-only and per-compositor, and
// Hyprland is the only compositor in the epic's target matrix that
// exposes it at all.
type LogindLockState struct {
	conn        *dbus.Conn
	sessionPath dbus.ObjectPath
	sigCh       chan *dbus.Signal
	lockCh      chan bool
	done        chan struct{}
	closeOnce   sync.Once
}

var _ LockState = (*LogindLockState)(nil)

// NewLogindLockState connects to the system bus, resolves the caller's
// logind session (via XDG_SESSION_ID when set, falling back to a lookup
// by the caller's own PID), reads its current LockedHint as the initial
// state, and starts watching it for lock-state changes.
//
// It watches two signal shapes on the session object, both scoped by
// sender to logind itself so an unprivileged local process cannot forge
// either one to force capture to (re)start while the real session is
// locked:
//
//   - Session.Lock / Session.Unlock, which fire only when something
//     calls the corresponding logind method (loginctl lock-session, a
//     lid-switch handler). Real lockers rarely go through these.
//   - Properties.PropertiesChanged for LockedHint, which is what
//     actually flips when a locker (hyprlock, swaylock, gtklock — none
//     of which call Session.Lock/Unlock themselves) engages via
//     ext_session_lock_manager_v1 or sets the hint directly. This is the
//     source that matters in practice and is treated as authoritative;
//     Lock/Unlock are a redundant fast path in case something in the
//     matrix does call them.
func NewLogindLockState(ctx context.Context) (*LogindLockState, error) {
	conn, err := dbus.ConnectSystemBus(dbus.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("session: connect system bus: %w", err)
	}

	sessionPath, err := resolveSessionPath(ctx, conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	sessObj := conn.Object(loginBusName, sessionPath)
	var lockedHint dbus.Variant
	if err := sessObj.CallWithContext(ctx, propertiesInterface+".Get", 0, sessionInterface, "LockedHint").Store(&lockedHint); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("session: read LockedHint: %w", err)
	}
	locked, _ := lockedHint.Value().(bool)

	if err := conn.AddMatchSignalContext(ctx,
		dbus.WithMatchObjectPath(sessionPath),
		dbus.WithMatchInterface(sessionInterface),
		dbus.WithMatchSender(loginBusName),
	); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("session: subscribe to session signals: %w", err)
	}
	if err := conn.AddMatchSignalContext(ctx,
		dbus.WithMatchObjectPath(sessionPath),
		dbus.WithMatchInterface(propertiesInterface),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchSender(loginBusName),
	); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("session: subscribe to property changes: %w", err)
	}

	sigCh := make(chan *dbus.Signal, 8)
	conn.Signal(sigCh)

	l := &LogindLockState{
		conn:        conn,
		sessionPath: sessionPath,
		sigCh:       sigCh,
		lockCh:      make(chan bool, 1),
		done:        make(chan struct{}),
	}
	l.lockCh <- locked
	go l.pump()
	return l, nil
}

// resolveSessionPath finds the logind object path for the process's own
// session. XDG_SESSION_ID is set by pam_systemd for every real login
// session and resolves directly; GetSessionByPID is the fallback for a
// process that inherited the environment from something other than a
// login session but still belongs to one, e.g. reattached to a cgroup by
// a session manager.
func resolveSessionPath(ctx context.Context, conn *dbus.Conn) (dbus.ObjectPath, error) {
	manager := conn.Object(loginBusName, loginObjectPath)
	var path dbus.ObjectPath

	if id := os.Getenv("XDG_SESSION_ID"); id != "" {
		if err := manager.CallWithContext(ctx, managerInterface+".GetSession", 0, id).Store(&path); err == nil {
			return path, nil
		}
	}

	if err := manager.CallWithContext(ctx, managerInterface+".GetSessionByPID", 0, uint32(os.Getpid())).Store(&path); err != nil {
		return "", fmt.Errorf("session: resolve logind session for pid %d: %w", os.Getpid(), err)
	}
	return path, nil
}

func (l *LogindLockState) Locked() <-chan bool { return l.lockCh }

// Close stops watching for signals and closes the D-Bus connection. It
// is safe to call more than once.
func (l *LogindLockState) Close() error {
	var err error
	l.closeOnce.Do(func() {
		close(l.done)
		l.conn.RemoveSignal(l.sigCh)
		err = l.conn.Close()
	})
	return err
}

// pump reads every signal delivered on the connection and folds the ones
// that matter — this session's Lock/Unlock — into lockCh.
func (l *LogindLockState) pump() {
	for {
		select {
		case <-l.done:
			return
		case sig, ok := <-l.sigCh:
			if !ok {
				return
			}
			if v, matched := decodeLockSignal(sig, l.sessionPath); matched {
				l.set(v)
			}
		}
	}
}

func (l *LogindLockState) set(v bool) {
	select {
	case <-l.lockCh:
	default:
	}
	l.lockCh <- v
}

// decodeLockSignal reports whether sig carries a lock-state update for
// the given session object — either a Lock/Unlock signal or a
// PropertiesChanged carrying a new LockedHint — and if so, the state it
// represents. It is factored out of pump so a test can drive it with
// canned *dbus.Signal values instead of a live system bus.
func decodeLockSignal(sig *dbus.Signal, sessionPath dbus.ObjectPath) (locked, matched bool) {
	if sig.Path != sessionPath {
		return false, false
	}
	switch sig.Name {
	case lockSignalName:
		return true, true
	case unlockSignalName:
		return false, true
	case propertiesChangedSignalName:
		return decodeLockedHintChange(sig)
	default:
		return false, false
	}
}

// decodeLockedHintChange extracts a new LockedHint value from a
// Properties.PropertiesChanged signal, per its documented body shape:
// (interface string, changed map[string]Variant, invalidated []string).
// It reports matched=false for anything that isn't a LockedHint change
// on org.freedesktop.login1.Session — including every other property
// PropertiesChanged reports, which this package has no use for.
func decodeLockedHintChange(sig *dbus.Signal) (locked, matched bool) {
	if len(sig.Body) < 2 {
		return false, false
	}
	iface, ok := sig.Body[0].(string)
	if !ok || iface != sessionInterface {
		return false, false
	}
	changed, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return false, false
	}
	v, ok := changed["LockedHint"]
	if !ok {
		return false, false
	}
	b, ok := v.Value().(bool)
	if !ok {
		return false, false
	}
	return b, true
}
