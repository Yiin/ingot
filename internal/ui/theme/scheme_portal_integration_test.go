//go:build integration

// These tests need a session bus they are allowed to own a well-known
// name on, so they are gated behind the integration tag — same convention
// as display_test.go in this package. scripts/headless.sh runs the suite
// under dbus-run-session, which gives each run a private bus with no real
// xdg-desktop-portal on it; that is what makes owning
// org.freedesktop.portal.Desktop possible here and impossible on a live
// desktop session (where they skip instead of failing).
//
// Unlike the rest of this package's integration tests they need no GDK
// display: Watch is called with a nil display on purpose, so apply only
// records the scheme and never touches a provider or a widget.
package theme

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// portalSettleDelay gives the match rule added inside Watch time to reach
// the bus daemon before the first emit. AddMatchSignal is a round trip to
// the daemon, but Watch does not wait on the goroutine that follows it,
// so an emit issued immediately can legitimately be dropped.
const portalSettleDelay = 200 * time.Millisecond

// signalWait bounds how long a single emit may take to come back around
// through the daemon and land in post.
const signalWait = 5 * time.Second

// impersonatePortal takes ownership of the portal's bus name for the
// duration of the test, so the test can emit SettingChanged itself, and
// returns the connection to emit on. Not owning the name is a skip, not a
// failure: it just means this ran against a session that already has a
// real portal.
func impersonatePortal(t *testing.T) *dbus.Conn {
	t.Helper()

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Skipf("no session bus: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// NameFlagDoNotQueue: fail immediately rather than sit in the queue
	// behind a real portal.
	reply, err := conn.RequestName(portalBusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		t.Skipf("cannot request %s: %v", portalBusName, err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		t.Skipf("%s is already owned (a real portal is running); this test needs to impersonate it", portalBusName)
	}
	t.Cleanup(func() { _, _ = conn.ReleaseName(portalBusName) })

	// apply writes package state that later tests in this binary read, so
	// put back whatever the suite's own theme.Load left active.
	t.Cleanup(OverrideScheme(ActiveScheme()))

	return conn
}

// emitColorScheme sends the portal's own SettingChanged for the
// appearance namespace, exactly as a real portal would.
func emitColorScheme(t *testing.T, conn *dbus.Conn, value uint32) {
	t.Helper()
	err := conn.Emit(portalObjectPath, portalChanged,
		appearanceNamespace, colorSchemeKey, dbus.MakeVariant(value))
	if err != nil {
		t.Fatalf("emitting SettingChanged(%d): %v", value, err)
	}
}

// TestWatchFollowsThePortal drives the whole live path end to end:
// Watch's real D-Bus subscription, its decode, its post to the "GTK
// thread", apply's palette swap, and finally stop's idempotency on a
// subscription that genuinely started. The unit tests in scheme_test.go
// cover the decode and precedence logic in isolation; only this one
// proves the parts are actually wired to each other.
func TestWatchFollowsThePortal(t *testing.T) {
	conn := impersonatePortal(t)

	// Watch reads the env var on every signal, so an inherited pin from
	// the developer's shell would make this test silently prove nothing.
	t.Setenv(SchemeEnvVar, "auto")

	applied := make(chan struct{}, 8)
	stop := Watch(nil, func(fn func()) {
		fn()
		applied <- struct{}{}
	})
	t.Cleanup(stop)

	time.Sleep(portalSettleDelay)

	steps := []struct {
		value uint32
		want  Scheme
	}{
		{1, SchemeDark},
		{2, SchemeLight},
		{1, SchemeDark},
	}
	for _, step := range steps {
		emitColorScheme(t, conn, step.value)

		select {
		case <-applied:
		case <-time.After(signalWait):
			t.Fatalf("SettingChanged(%d) never reached Watch within %v — the match rule never fired", step.value, signalWait)
		}

		if got := ActiveScheme(); got != step.want {
			t.Fatalf("after SettingChanged(%d), ActiveScheme() = %v, want %v", step.value, got, step.want)
		}
	}

	// The claim stop actually has to carry: idempotent on a subscription
	// that really started, and not deadlocking on its own goroutine.
	done := make(chan struct{})
	go func() {
		defer close(done)
		stop()
		stop()
	}()
	select {
	case <-done:
	case <-time.After(signalWait):
		t.Fatal("stop() hung")
	}
}

// TestWatchIgnoresThePortalWhenPinned is the negative half of the env
// contract: INGOT_COLOR_SCHEME is a pin, so a desktop that flips must not
// move it. It has to live here rather than beside the unit tests because
// the only way to show a signal was ignored is to actually send one — a
// unit test could only observe that Watch returned, which stays true even
// with the pin check deleted.
//
// "post was never called" is not proven by waiting alone. The test also
// subscribes to the same signal on its own connection and waits to see it
// come back through the daemon: once the daemon has dispatched it to a
// matching subscriber, a Watch that had subscribed would have had it too.
func TestWatchIgnoresThePortalWhenPinned(t *testing.T) {
	conn := impersonatePortal(t)

	t.Setenv(SchemeEnvVar, "light")
	t.Cleanup(OverrideScheme(SchemeLight))

	posted := make(chan struct{}, 8)
	stop := Watch(nil, func(fn func()) {
		fn()
		posted <- struct{}{}
	})
	if stop == nil {
		t.Fatal("Watch returned a nil stop")
	}
	t.Cleanup(stop)

	// The control subscription — see the doc comment above.
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(portalObjectPath),
		dbus.WithMatchInterface(portalInterface),
		dbus.WithMatchMember("SettingChanged"),
	); err != nil {
		t.Fatalf("subscribing the control receiver: %v", err)
	}
	control := make(chan *dbus.Signal, 8)
	conn.Signal(control)
	t.Cleanup(func() { conn.RemoveSignal(control) })

	time.Sleep(portalSettleDelay)

	// 1 is dark: without the pin this would flip the scheme, so a scheme
	// that does not move is the pin doing its job.
	emitColorScheme(t, conn, 1)

	select {
	case <-control:
	case <-time.After(signalWait):
		t.Fatalf("the control receiver never saw SettingChanged within %v — the emit never reached the daemon, so this test proved nothing", signalWait)
	}

	// The daemon has dispatched. Give a posted callback the same room to
	// land before concluding none is coming.
	select {
	case <-posted:
		t.Fatal("Watch posted a scheme change despite INGOT_COLOR_SCHEME=light")
	case <-time.After(portalSettleDelay):
	}

	if got := ActiveScheme(); got != SchemeLight {
		t.Errorf("ActiveScheme() = %v after a dark SettingChanged, want %v (the env pin must win)", got, SchemeLight)
	}
}
