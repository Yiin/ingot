package theme

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/godbus/dbus/v5"
)

// Scheme is which of the two palettes the panel is currently painted in.
type Scheme int

const (
	SchemeLight Scheme = iota
	SchemeDark
)

// String renders a Scheme as the same word SchemeEnvVar accepts, so a log
// line and the override that would reproduce it read the same.
func (s Scheme) String() string {
	if s == SchemeDark {
		return "dark"
	}
	return "light"
}

// SchemeEnvVar pins the colour scheme regardless of what the desktop
// says: "light", "dark", or "auto". Anything else (including unset) means
// auto. A pinned value also stops Watch reacting to portal changes — a
// pin the desktop can override is not a pin.
const SchemeEnvVar = "INGOT_COLOR_SCHEME"

// The xdg-desktop-portal appearance setting. This is the one signal that
// actually works here: GtkSettings:gtk-application-prefer-dark-theme
// reads false on this machine's dark desktop, so it is not usable, while
// ReadOne("org.freedesktop.appearance", "color-scheme") correctly returns
// 1 (dark).
const (
	portalBusName    = "org.freedesktop.portal.Desktop"
	portalObjectPath = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	portalInterface  = "org.freedesktop.portal.Settings"
	portalReadOne    = portalInterface + ".ReadOne"
	portalChanged    = portalInterface + ".SettingChanged"

	appearanceNamespace = "org.freedesktop.appearance"
	colorSchemeKey      = "color-scheme"
)

// portalPref is the portal's color-scheme value, with an extra
// "unavailable" state for "no portal, or it answered something this code
// cannot read".
type portalPref int

const (
	portalUnavailable  portalPref = -1
	portalNoPreference portalPref = 0
	portalPrefersDark  portalPref = 1
	portalPrefersLight portalPref = 2
)

// portalReadTimeout bounds the one blocking portal call Load makes. This
// budget is the panel's startup latency, not just the call's: Load runs
// inline on the GTK thread before any widget exists, so nothing appears
// on screen until this returns. A local D-Bus round trip answers in
// single-digit milliseconds, so 500ms is already two orders of magnitude
// of headroom, and expiring is cheap — detection falls through to the
// GTK theme name and then to SchemeLight, which is the palette the whole
// stylesheet is written in anyway.
const portalReadTimeout = 500 * time.Millisecond

// darkProviderBoost is how far above STYLE_PROVIDER_PRIORITY_APPLICATION
// the dark override sits: one step, which is enough to win against the
// base sheet's :root and still lose to a user's own provider.
const darkProviderBoost uint = 1

// active* is the resolved scheme and the palette that goes with it. There
// is deliberately no mutex: everything that reads them (Colors and
// ActiveScheme, from cairo DrawFuncs) and everything that writes them
// (apply, from Load or from inside the caller's post callback) runs on
// the GTK thread, which internal/ui/gtkapp's package doc already
// establishes as the only thread allowed to touch GTK state at all. A
// mutex here would suggest the rest of internal/ui is safe to call from
// anywhere, which it is not.
var (
	activeScheme  = SchemeLight
	activePalette = Light
)

// darkProvider is built once and reused, because installing a scheme is
// not a one-shot: the user can flip their desktop back and forth and each
// flip adds or removes this same provider.
var (
	darkProvider *gtk.CSSProvider
	darkAdded    bool
)

// Colors returns the palette of the active scheme, for the cairo
// DrawFuncs that paint outside GTK's CSS engine and so cannot resolve a
// var(). Call it from the GTK thread only, at draw time rather than at
// widget-construction time — see the note on active* above, and note that
// a cached copy would silently keep painting the old scheme.
func Colors() Palette { return activePalette }

// ActiveScheme returns the scheme Load or the last portal change
// resolved. GTK thread only, same as Colors.
func ActiveScheme() Scheme { return activeScheme }

// OverrideScheme forces the active palette to s until the returned
// restore func is called, so a test can exercise both schemes without a
// display, a session bus, or a desktop set to the scheme it wants. It is
// the same test-seam idiom internal/ui/motion uses for
// OverrideEnableAnimations, and carries the same caveat: not safe for
// concurrent use, which costs nothing because every caller of Colors is
// on the GTK thread anyway.
//
// It passes a nil display deliberately: with no display, apply swaps the
// package's palette and skips both the CSS provider and the repaint, so
// nothing here depends on GTK being initialised.
func OverrideScheme(s Scheme) (restore func()) {
	prev := activeScheme
	apply(nil, s)
	return func() { apply(nil, prev) }
}

// resolveScheme folds the three detection signals into one scheme, in
// precedence order: the env pin, then the portal, then the GTK theme
// name's "-dark" suffix, then light. It is a pure function so the whole
// precedence table is testable without a display or a session bus; the
// three thin IO wrappers below are what actually fetch the arguments.
func resolveScheme(env string, portal portalPref, gtkTheme string) Scheme {
	if s, pinned := pinnedScheme(env); pinned {
		return s
	}
	switch portal {
	case portalPrefersDark:
		return SchemeDark
	case portalPrefersLight:
		return SchemeLight
	}
	// gtk-application-prefer-dark-theme is not consulted: it reads false
	// even under Adwaita-dark, so the theme name is the only GtkSettings
	// signal worth anything here.
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(gtkTheme)), "-dark") {
		return SchemeDark
	}
	return SchemeLight
}

// pinnedScheme reports whether SchemeEnvVar names one specific scheme.
// "auto", an empty value and anything unrecognised all mean "not pinned",
// so a typo degrades to detection rather than to an arbitrary scheme.
func pinnedScheme(env string) (Scheme, bool) {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "light":
		return SchemeLight, true
	case "dark":
		return SchemeDark, true
	default:
		return SchemeLight, false
	}
}

// detectScheme runs the full precedence chain against the live system.
// The portal read is skipped when the env var already pins the answer, so
// a pinned run never waits on a session bus that may not be there.
func detectScheme() Scheme {
	env := os.Getenv(SchemeEnvVar)
	portal := portalUnavailable
	if _, pinned := pinnedScheme(env); !pinned {
		portal = readPortalScheme()
	}
	return resolveScheme(env, portal, gtkThemeName())
}

// readPortalScheme asks xdg-desktop-portal for the current colour scheme.
// Every failure — no session bus, no portal, an answer of an unexpected
// shape — returns portalUnavailable so detection falls through to the
// next signal instead of failing.
func readPortalScheme() portalPref {
	ctx, cancel := context.WithTimeout(context.Background(), portalReadTimeout)
	defer cancel()

	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		slog.Debug("theme: no session bus for colour-scheme detection", "err", err)
		return portalUnavailable
	}
	defer func() { _ = conn.Close() }()

	obj := conn.Object(portalBusName, portalObjectPath)
	var out dbus.Variant
	// FlagNoAutoStart is load-bearing, not a micro-optimisation. Without
	// it this call D-Bus-activates xdg-desktop-portal whenever one is not
	// already running, which starts a whole portal stack
	// (xdg-desktop-portal plus its -gtk and compositor backends). Those
	// open real toplevels, and in a bare session sharing one compositor
	// they take keyboard focus: measured, that is what made
	// internal/e2e's TestRun_StartsMapsCapturesAndSingleInstances fail
	// roughly half its runs, its wtype keystrokes landing in a portal
	// window instead of the panel. Reading an existing desktop preference
	// is never worth spawning a service to answer it, so when no portal
	// is already up, fall through to the gtk-theme-name signal instead.
	if err := obj.CallWithContext(ctx, portalReadOne, dbus.FlagNoAutoStart, appearanceNamespace, colorSchemeKey).Store(&out); err != nil {
		slog.Debug("theme: portal colour-scheme read failed", "err", err)
		return portalUnavailable
	}
	pref, ok := decodePortalPref(out)
	if !ok {
		return portalUnavailable
	}
	return pref
}

// decodePortalPref unwraps the portal's answer. ReadOne is declared to
// return a plain variant, but portal implementations in the wild nest the
// value one variant deeper, so unwrap until something that is not a
// variant falls out. Anything that is not then a uint32 is reported as
// unreadable rather than guessed at.
func decodePortalPref(v any) (portalPref, bool) {
	// Bounded rather than unbounded: a real answer is nested at most once,
	// and the extra headroom only exists so a stranger implementation
	// still resolves instead of being rejected. It must not be a `for {}`
	// — dbus.Variant can hold itself, and an unbounded unwrap of a
	// malicious value would spin forever.
	for range 4 {
		inner, ok := v.(dbus.Variant)
		if !ok {
			break
		}
		v = inner.Value()
	}
	u, ok := v.(uint32)
	if !ok {
		return portalUnavailable, false
	}
	switch portalPref(u) {
	case portalNoPreference, portalPrefersDark, portalPrefersLight:
		return portalPref(u), true
	default:
		return portalUnavailable, false
	}
}

// decodeSettingChanged reports whether sig is the appearance namespace's
// color-scheme change, and the value it carries. Split out of Watch's
// goroutine so the signal body shape is testable without a session bus.
func decodeSettingChanged(sig *dbus.Signal) (portalPref, bool) {
	if sig == nil || sig.Name != portalChanged || len(sig.Body) < 3 {
		return portalUnavailable, false
	}
	namespace, ok := sig.Body[0].(string)
	if !ok || namespace != appearanceNamespace {
		return portalUnavailable, false
	}
	key, ok := sig.Body[1].(string)
	if !ok || key != colorSchemeKey {
		return portalUnavailable, false
	}
	return decodePortalPref(sig.Body[2])
}

// gtkThemeName reads GtkSettings:gtk-theme-name, or "" when there is no
// default settings object (no display, e.g. under go test). GTK thread
// only, like every other GtkSettings read in internal/ui.
func gtkThemeName() string {
	settings := gtk.SettingsGetDefault()
	if settings == nil {
		return ""
	}
	name, ok := settings.ObjectProperty("gtk-theme-name").(string)
	if !ok {
		return ""
	}
	return name
}

// darkOverrideCSS is the whole dark stylesheet: a :root block redefining
// every colour custom property, and nothing else. There is no second
// .css file because there is no second set of rules — style.css's
// selectors already resolve every colour through var(), so overriding the
// properties at a higher provider priority repaints the entire panel.
//
// Properties are emitted in sorted order so two calls produce byte-equal
// output and a diff of the generated sheet is readable.
func darkOverrideCSS() string {
	tokens := Dark.tokens()
	names := make([]string, 0, len(tokens))
	for name := range tokens {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString(":root {\n")
	for _, name := range names {
		fmt.Fprintf(&b, "  %s: %s;\n", name, tokens[name])
	}
	b.WriteString("}\n")
	return b.String()
}

// apply installs or removes the dark override, records the active
// palette, and repaints. Safe to call when Load failed and the base
// provider was never installed (internal/app only warns on that): the
// override provider is independent of the base one, and a display that
// never got the base sheet simply has nothing for these properties to
// cascade into.
func apply(display *gdk.Display, s Scheme) {
	if display != nil {
		if s == SchemeDark {
			addDarkProvider(display)
		} else {
			removeDarkProvider(display)
		}
	}

	activeScheme = s
	if s == SchemeDark {
		activePalette = Dark
	} else {
		activePalette = Light
	}

	if display == nil {
		return // no display means no toplevels to repaint
	}
	repaintAllToplevels()
}

// addDarkProvider installs the override one step above
// STYLE_PROVIDER_PRIORITY_APPLICATION. A second provider redefining only
// :root custom properties does override the base provider's values, and
// removing it reverts them — verified under headless sway by reading
// gtk_widget_get_color across an add/remove cycle.
func addDarkProvider(display *gdk.Display) {
	if darkAdded {
		return
	}
	if darkProvider == nil {
		darkProvider = gtk.NewCSSProvider()
		// Deliberately not connecting ConnectParsingError — see theme.go's
		// comment on the double-free in gotk4 v0.4.0.
		darkProvider.LoadFromString(darkOverrideCSS())
	}
	gtk.StyleContextAddProviderForDisplay(display, darkProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION+darkProviderBoost)
	darkAdded = true
}

func removeDarkProvider(display *gdk.Display) {
	if !darkAdded || darkProvider == nil {
		return
	}
	gtk.StyleContextRemoveProviderForDisplay(display, darkProvider)
	darkAdded = false
}

// repaintAllToplevels queues a redraw on every widget of every toplevel,
// not just on the toplevels themselves.
//
// One QueueDraw per toplevel is not enough. GTK4 caches a widget's render
// node and reuses it when an ancestor repaints, so an ancestor's
// QueueDraw does not re-run a descendant GtkDrawingArea's DrawFunc — and
// the DrawFuncs are exactly what needs to run again, since the cairo
// drawers in internal/ui/widget, internal/ui/notelist and
// internal/ui/toast read Colors() at draw time.
//
// Walking the tree is also what reaches the in-panel toast: it is a
// GtkRevealer inside the panel window (internal/ui/toast/inpanel.go:50),
// not a toplevel of its own, so a toplevel-only loop would leave its
// hand-drawn check icon painted in the previous scheme.
func repaintAllToplevels() {
	toplevels := gtk.WindowGetToplevels()
	if toplevels == nil {
		return
	}
	for i := uint(0); i < toplevels.NItems(); i++ {
		obj := toplevels.Item(i)
		if obj == nil {
			continue
		}
		w, ok := obj.Cast().(gtk.Widgetter)
		if !ok {
			continue
		}
		queueDrawTree(w)
	}
}

func queueDrawTree(w gtk.Widgetter) {
	base := gtk.BaseWidget(w)
	base.QueueDraw()
	for child := base.FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
		queueDrawTree(child)
	}
}

// Watch subscribes to the portal's appearance changes and re-applies the
// scheme whenever the desktop flips. post is the caller's GTK-thread
// marshal (internal/ui/gtkapp's App.Post): every GTK and GObject call
// this makes happens inside it, because the signal arrives on a plain
// goroutine and internal/ui/gtkapp's package doc allows exactly one
// thread to touch GTK.
//
// The returned stop is idempotent and always safe to call, including when
// the portal was never reachable and nothing was ever started.
func Watch(display *gdk.Display, post func(func())) (stop func()) {
	noop := func() {}
	if post == nil {
		return noop
	}
	// A pinned scheme is a pin: the desktop flipping must not move it.
	if _, pinned := pinnedScheme(os.Getenv(SchemeEnvVar)); pinned {
		return noop
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		slog.Debug("theme: no session bus, colour scheme will not follow the desktop", "err", err)
		return noop
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(portalObjectPath),
		dbus.WithMatchInterface(portalInterface),
		dbus.WithMatchMember("SettingChanged"),
		dbus.WithMatchSender(portalBusName),
	); err != nil {
		slog.Debug("theme: cannot subscribe to portal appearance changes", "err", err)
		_ = conn.Close()
		return noop
	}

	sigCh := make(chan *dbus.Signal, 8)
	conn.Signal(sigCh)

	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-done:
				return
			case sig, ok := <-sigCh:
				if !ok {
					return
				}
				pref, matched := decodeSettingChanged(sig)
				if !matched {
					continue
				}
				// gtkThemeName and apply are both GTK calls, so the whole
				// resolve-and-apply step goes across post, not just apply.
				post(func() { apply(display, resolveScheme(os.Getenv(SchemeEnvVar), pref, gtkThemeName())) })
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			// Stop the goroutine first: closing the connection while it is
			// still reading would race the connection's own signal dispatch.
			close(done)
			<-stopped
			conn.RemoveSignal(sigCh)
			_ = conn.Close()
		})
	}
}
