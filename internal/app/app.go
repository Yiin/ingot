package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/Yiin/ingot/internal/config"
	"github.com/Yiin/ingot/internal/hotkey"
	"github.com/Yiin/ingot/internal/input"
	"github.com/Yiin/ingot/internal/selection"
	"github.com/Yiin/ingot/internal/session"
	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsstore"
	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/paths"
	"github.com/Yiin/ingot/internal/ui/gtkapp"
	"github.com/Yiin/ingot/internal/ui/keymap"
	"github.com/Yiin/ingot/internal/ui/menus"
	"github.com/Yiin/ingot/internal/ui/panel"
	"github.com/Yiin/ingot/internal/ui/theme"
	"github.com/Yiin/ingot/internal/ui/toast"
)

// AppID is Ingot's D-Bus/GtkApplication identifier.
const AppID = "lt.yiin.ingot"

// Options configures Run.
type Options struct {
	// Hidden starts the panel unmapped — for a systemd user unit that
	// should not steal focus at login.
	Hidden bool
}

// App is one running Ingot process: the store, the panel, and every
// background goroutine wired together. See the package doc for the
// threading rule every method here depends on.
type App struct {
	layout paths.Layout
	cfg    config.Config
	gapp   *gtkapp.App

	store   store.Store
	adapter *storeAdapter
	unsub   func()

	shell   *panel.Shell
	win     *gtk.ApplicationWindow
	toaster *toast.Toaster

	detector *hotkey.Detector
	reader   selection.Reader

	src         input.Source
	lock        session.LockState
	gateCancel  context.CancelFunc
	schemeStop  func()
	stopSignals context.CancelFunc

	menuActions *menus.Actions
	ctxMenu     *menus.ContextMenuController
	nav         *keymap.Nav
	// syncingNavToList guards onListSelectionChanged against reacting to
	// its own syncNavToList's SelectItems call — see nav.go.
	syncingNavToList bool
	// panelState is panel.json in memory, the single writer for it. Held
	// whole rather than field by field because SetKeepOnTop and
	// savePanelSize both persist it, and either writing only its own field
	// would blank the other's.
	panelState config.PanelState
	// started guards the activate handler: GApplication fires activate once
	// during Run, and startup has already decided whether the panel begins
	// visible, so only a later activate (a re-launch) should present it.
	started bool
	// releaseHold drops the GApplication hold taken in startup. Ingot must
	// outlive its hidden panel so the capture chord stays armed.
	releaseHold func()
	// keyOverrides is config.toml's [keys] section, narrowed by
	// applyKeyOverrides to just the entries keymap.ApplyOverrides
	// accepted — see menus.go.
	keyOverrides map[string]string

	hidden  bool
	visible bool

	shutdownOnce sync.Once
}

// Run resolves paths and config, becomes (or hands off to) the single
// Ingot instance, and blocks until the process exits, returning the
// process exit code.
func Run(opts Options) int {
	layout, err := paths.Resolve()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ingot: run:", err)
		return 1
	}

	cfg, warnings, err := config.Load(fsx.OS(), layout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ingot: run: config:", err)
	}
	for _, w := range warnings {
		slog.Warn("ingot: config.toml: " + w)
	}

	panelState := config.LoadPanelState(fsx.OS(), layout)

	a := &App{
		layout:     layout,
		cfg:        cfg,
		hidden:     opts.Hidden,
		detector:   hotkey.NewDetector(cfg.Hotkey.Window),
		panelState: panelState,
	}

	// Build the real App first and probe the bus through it. Registering a
	// separate throwaway GApplication under the same id would export the
	// org.gtk.Application object at this id's path, and the real one could
	// then never export there — no primary instance could start at all.
	a.gapp = gtkapp.New(AppID)

	a.gapp.AddAction("toggle", func() { a.toggle() })

	// GApplication requires an activate handler or Run warns and returns
	// straight away. Startup already decided whether the panel begins
	// visible, so the first activate must not fight that; only a later one
	// (a re-launch from the desktop file) presents the panel.
	a.gapp.OnActivate(func(*gtkapp.App) {
		if a.started {
			a.show()
		}
		a.started = true
	})
	a.gapp.OnStartup(func(*gtkapp.App) {
		if err := a.startup(); err != nil {
			fmt.Fprintln(os.Stderr, "ingot: run:", err)
			os.Exit(1)
		}
	})
	a.gapp.ConnectShutdown(func() { a.shutdown() })

	// Register only now, with every handler already connected.
	// g_application_register emits "startup" synchronously the moment this
	// process becomes the primary instance, so registering any earlier
	// would fire it before OnStartup is attached: the panel would never be
	// built, nothing would hold the application open, and Run would return
	// 0 immediately having done nothing.
	sentRemote, err := a.gapp.TryActivateRemote(context.Background(), "toggle")
	if err != nil {
		fmt.Fprintln(os.Stderr, "ingot: run:", err)
		return 1
	}
	if sentRemote {
		return 0
	}

	return a.gapp.Run()
}

// startup builds every widget and wires every flow. It runs on the GTK
// thread, inside gtkapp's OnStartup callback — see gtkapp's own doc
// comment on init order.
func (a *App) startup() error {
	// Must run before any wire* call below reads keymap.Table (directly,
	// via bindTableAction, or through Resolve inside InstallNav) — see
	// applyKeyOverrides' own doc comment.
	a.applyKeyOverrides()

	if err := theme.Load(gdk.DisplayGetDefault()); err != nil {
		slog.Warn("app: theme.Load failed, styling degraded", "err", err)
	}
	// theme.Load already applied the scheme the desktop reports now; this
	// keeps following it. The portal signal arrives on its own goroutine,
	// so Post is what carries the restyle back to the GTK thread.
	a.schemeStop = theme.Watch(gdk.DisplayGetDefault(), a.gapp.Post)

	toaster, err := toast.New(slog.Default())
	if err != nil {
		return fmt.Errorf("build notifier: %w", err)
	}
	a.toaster = toaster

	st, err := fsstore.New(fsstore.Options{
		FS:    fsx.OS(),
		Paths: a.layout,
		Post:  a.gapp.Post,
		Watch: true,
	})
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	a.store = st

	if len(st.Projects()) == 0 {
		if _, err := st.CreateProject("Notes"); err != nil {
			return fmt.Errorf("seed first project: %w", err)
		}
	}

	proj, err := st.Project(st.Active())
	if err != nil {
		return fmt.Errorf("load active project: %w", err)
	}

	a.shell = panel.New(storeSectionsToNotelist(proj.Sections), proj.Title, a.toaster.Panel(), a.toaster)

	a.adapter = newStoreAdapter(a.store, a.shell.List().Model(), a.toaster.Message)
	a.adapter.Seed()
	a.unsub = a.store.Subscribe(safeEvent("store-event", a.adapter.OnEvent))

	// An ordinary toplevel, not a layer-shell surface: the panel is meant
	// to be moved, resized and fullscreened like any other window, and an
	// overlay layer surface can do none of those. It also held the
	// keyboard for as long as it was mapped, so clicking another window
	// never really handed focus back.
	//
	// Undecorated, because the compositor already frames it. GTK's own
	// alternative is a CSD headerbar, which costs vertical space and looks
	// unlike anything else on a wlroots desktop. Wayland gives a client no
	// say in its own placement, so where the panel opens is up to the
	// compositor — float it with a window rule against the "lt.yiin.ingot"
	// app id (README, "The panel window").
	win := gtk.NewApplicationWindow(a.gapp.Application)
	win.SetChild(a.shell.Widget())
	win.SetTitle("Ingot")
	win.SetDecorated(false)
	win.SetDefaultSize(a.panelSize())
	// Paints the window the panel's colour, so a resize cannot flash the
	// system theme's grey through before the child catches up.
	win.AddCSSClass(theme.PanelWindowClass)
	a.win = win

	// The panel is now genuinely unfocused whenever another window is
	// active, so the dimmed focus-ring state tracks the real thing instead
	// of being driven by show/hide. As a layer surface it was focused for
	// its whole visible life and this distinction could not arise.
	win.NotifyProperty("is-active", func() {
		a.shell.SetFocused(win.IsActive())
	})

	a.wireCompose()
	a.wireCopyShortcuts()
	a.wireListGate()
	a.wireListToggle()
	a.wireMenus()
	a.wireNav()
	// After wireMenus: the cascade's first step closes the context menu
	// popover, which wireMenus is what creates.
	a.wireEscape()

	win.ConnectCloseRequest(func() bool {
		defer guard("close-request")()
		a.hide()
		return true
	})

	a.startChord()

	// Outlive the panel. GApplication quits once its last visible window
	// goes away, so without this hold, hiding the panel would end the
	// process and disarm the capture chord — the exact state Ingot is
	// meant to sit in most of the time. shutdown drops it.
	a.releaseHold = a.gapp.KeepAlive()

	sigCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	a.stopSignals = stopSignals
	goSafe("signals", func() {
		<-sigCtx.Done()
		a.gapp.Post(func() { a.shutdown() })
	})

	if !a.hidden {
		a.show()
	}

	return nil
}

// notifier returns the app's Notifier.
func (a *App) notifier() toast.Notifier { return a.toaster }

// panelSize returns the size the panel window should open at: whatever
// the user last left it, falling back to the design's own 360x640.
func (a *App) panelSize() (width, height int) {
	width, height = a.panelState.Width, a.panelState.Height
	if width <= 0 {
		width = theme.PanelWidth
	}
	if height <= 0 {
		height = theme.PanelHeight
	}
	return width, height
}

// savePanelSize records the panel window's current size so the next
// launch opens at it.
//
// Fullscreen is deliberately not recorded: it is a temporary mode rather
// than a chosen size, and persisting it would leave the panel opening
// full-screen forever after a single Super+F, with no affordance to get
// the old size back.
//
// Maximised deliberately IS recorded, even though it reads like the same
// case. GtkWindow.IsMaximized reports the xdg_toplevel maximized state,
// which wlroots compositors set on any window they size themselves — on
// Hyprland it was true for a plain floating 640x900 window that the user
// had never maximised (measured: mapped=true max=true fs=false w=640
// h=900). Excluding it therefore did not exclude "the user maximised
// this", it disabled size persistence outright on the compositors Ingot
// targets. A genuinely maximised window reopening at that size is the
// mild and defensible failure here.
func (a *App) savePanelSize() {
	if a.win == nil || !a.win.Mapped() {
		return
	}
	if a.win.IsFullscreen() {
		return
	}
	w, h := a.win.Width(), a.win.Height()
	if w <= 0 || h <= 0 {
		return
	}
	if w == a.panelState.Width && h == a.panelState.Height {
		return
	}
	a.panelState.Width, a.panelState.Height = w, h
	a.savePanelState()
}

// savePanelState writes panel.json. A failure here costs a preference,
// never data, so it warns rather than propagating.
func (a *App) savePanelState() {
	if err := config.SavePanelState(fsx.OS(), a.layout, a.panelState); err != nil {
		slog.Warn("app: save panel state", "err", err)
	}
}
