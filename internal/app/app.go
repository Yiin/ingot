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
	"github.com/Yiin/ingot/internal/layershell"
	"github.com/Yiin/ingot/internal/selection"
	"github.com/Yiin/ingot/internal/session"
	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsstore"
	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/paths"
	"github.com/Yiin/ingot/internal/ui/gtkapp"
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
	lsPanel *layershell.Panel
	toaster *toast.Toaster

	detector *hotkey.Detector
	reader   selection.Reader

	src         input.Source
	lock        session.LockState
	gateCancel  context.CancelFunc
	stopSignals context.CancelFunc

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

	sentRemote, err := gtkapp.ToggleRemote(context.Background(), AppID, "toggle")
	if err != nil {
		fmt.Fprintln(os.Stderr, "ingot: run:", err)
		return 1
	}
	if sentRemote {
		return 0
	}

	a := &App{layout: layout, cfg: cfg, hidden: opts.Hidden, detector: hotkey.NewDetector(cfg.Hotkey.Window)}

	a.gapp = gtkapp.New(AppID)
	a.gapp.AddAction("toggle", func() { a.toggle() })
	a.gapp.OnStartup(func(*gtkapp.App) {
		if err := a.startup(); err != nil {
			fmt.Fprintln(os.Stderr, "ingot: run:", err)
			os.Exit(1)
		}
	})
	a.gapp.ConnectShutdown(func() { a.shutdown() })

	return a.gapp.Run()
}

// startup builds every widget and wires every flow. It runs on the GTK
// thread, inside gtkapp's OnStartup callback — see gtkapp's own doc
// comment on init order.
func (a *App) startup() error {
	if err := theme.Load(gdk.DisplayGetDefault()); err != nil {
		slog.Warn("app: theme.Load failed, styling degraded", "err", err)
	}

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

	win := gtk.NewApplicationWindow(a.gapp.Application)
	win.SetChild(a.shell.Widget())
	win.SetDecorated(false)
	a.win = win

	lsPanel, err := layershell.New(&win.Window, layershell.DefaultConfig(), nil)
	if err != nil {
		return fmt.Errorf("layer-shell surface (compositor may lack wlr-layer-shell; run `ingot doctor`): %w", err)
	}
	a.lsPanel = lsPanel

	a.wireCompose()
	a.wireCopyShortcuts()
	a.wireListGate()
	a.wireListToggle()

	win.ConnectCloseRequest(func() bool {
		defer guard("close-request")()
		a.hide()
		return true
	})

	a.startChord()

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
