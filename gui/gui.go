package gui

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"focusd/coreapi"
	"focusd/storage"
	"focusd/system"
	"log"
	"os"
	"strings"
	"time"
)

// Run opens the desktop interface. The tracker itself lives in the daemon; the
// GUI is a read/write client over the same SQLite store and config files, so it
// can be opened and closed without interrupting tracking.
func Run() error {
	if err := storage.Init(); err != nil {
		return err
	}
	defer storage.Close()

	app := newApp()
	url, shutdown, err := startServer(app)
	if err != nil {
		return err
	}
	defer shutdown()

	if err := openWindow(url, app); err != nil {
		if errors.Is(err, errAlreadyRunning) {
			return nil
		}
		if errors.Is(err, errBrowserFallback) {
			waitForBrowserSession(app)
			return nil
		}
		return err
	}

	// Persist the last partial session before the process goes away. The daemon
	// also flushes, but it must not be the only writer that saves this.
	coreapi.SendIPCCmd("flush")
	return nil
}

func newApp() *app {
	a := &app{
		token:   randomToken(),
		version: system.Version,
		debug:   strings.TrimSpace(os.Getenv("FOCUSD_GUI_DEBUG")) == "1",
	}
	a.touch()
	a.prepareVersionNotice()
	return a
}

// prepareVersionNotice detects an upgrade or first run and asks the UI to
// remind the user to reload the browser extension once.
func (a *app) prepareVersionNotice() {
	last, _ := storage.GetConfig("last_run_version")
	pending, _ := storage.GetConfig("extension_reload_pending")
	a.extensionReload = pending == "1"
	if last != a.version {
		a.extensionReload = true
		_ = storage.SetConfig("extension_reload_pending", "1")
		_ = storage.SetConfig("last_run_version", a.version)
	}
}

// touch marks the page as alive. The browser fallback uses this heartbeat so
// the local server stays up while a tab is open and shuts down shortly after
// the tab is closed.
func (a *app) touch() {
	a.lastSeen.Store(time.Now().UnixNano())
}

func waitForBrowserSession(a *app) {
	const idleTimeout = 3 * time.Minute
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		last := time.Unix(0, a.lastSeen.Load())
		if time.Since(last) > idleTimeout {
			return
		}
	}
}

func randomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing is fatal-adjacent territory; a predictable token
		// would silently weaken a localhost service, so refuse rather than
		// fall back to math/rand.
		log.Fatalf("gui: cannot generate session token: %v", err)
	}
	return hex.EncodeToString(buf)
}

var (
	errNoWindow        = errors.New("no supported window backend")
	errBrowserFallback = errors.New("running in the default browser")
	errAlreadyRunning  = errors.New("DeepFocus is already running")
)
