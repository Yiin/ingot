package fsstore

import (
	"time"

	"github.com/Yiin/ingot/internal/store"
	"github.com/Yiin/ingot/internal/store/fsx"
	"github.com/Yiin/ingot/internal/store/paths"
)

// defaultDebounce and defaultMaxDelay are the idle-debounce and capped
// maximum delay for persisting a project after its last change, used
// whenever Options leaves the corresponding field at its zero value.
const (
	defaultDebounce = 250 * time.Millisecond
	defaultMaxDelay = 2 * time.Second
)

// Options configures a Store returned by New.
type Options struct {
	// FS is the filesystem seam every read and write goes through.
	FS fsx.FS
	// Paths is the directory layout to load from and save to.
	Paths paths.Layout
	// Now returns the current time, used for every Created/DoneAt
	// timestamp the Store mints. Defaults to time.Now.
	Now func() time.Time
	// NewID mints a fresh id for every Project, Section, and Note the
	// Store creates at runtime (not ones minted by mdfile.Parse while
	// loading). Defaults to store.NewID.
	NewID func() string
	// Post schedules fn to run on the goroutine that constructed the
	// Store — in the app, glib.IdleAdd onto the GTK main loop. It is
	// used only to hop a debounced save, fired from a background timer,
	// back onto that goroutine. Defaults to running fn synchronously,
	// on the timer's own goroutine, which is adequate for tests and any
	// caller with no single-goroutine requirement of its own.
	Post func(func())
	// Debounce is how long a project must sit unchanged before its
	// pending save is written. Defaults to 250ms.
	Debounce time.Duration
	// MaxDelay caps how long a continuously-edited project can go
	// without a save, regardless of how often Debounce keeps resetting.
	// Defaults to 2s.
	MaxDelay time.Duration
	// Watch is accepted and ignored: file watching is a separate
	// package addition. Reserved so Options doesn't need to change
	// shape when it lands.
	Watch bool
}

func (o Options) withDefaults() Options {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.NewID == nil {
		o.NewID = store.NewID
	}
	if o.Post == nil {
		o.Post = func(fn func()) { fn() }
	}
	if o.Debounce <= 0 {
		o.Debounce = defaultDebounce
	}
	if o.MaxDelay <= 0 {
		o.MaxDelay = defaultMaxDelay
	}
	return o
}
