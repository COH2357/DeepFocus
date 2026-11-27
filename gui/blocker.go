package gui

import (
	"focusd/storage"
	"strings"
)

// BlockOptions describes the one limit that triggered this blocker process.
type BlockOptions struct {
	Kind         string
	Target       string
	Label        string
	LimitMinutes int
	UsedSeconds  int
	HWND         uintptr
	PID          uint32
	Unit         string
	MaxOpens     int
}

// RunBlocker is the entry point for `deepfocus_gui.exe --block`. It starts a
// small local server and a dedicated topmost window. The blocker is a separate
// process so it still appears when the main DeepFocus window is closed.
func RunBlocker(opts BlockOptions) error {
	if opts.Unit == "" {
		opts.Unit = "minutes"
	}
	if err := storage.Init(); err != nil {
		return err
	}
	defer storage.Close()

	a := newApp()
	a.block = &opts
	url, shutdown, err := startServer(a)
	if err != nil {
		return err
	}
	defer shutdown()

	blockURL := strings.Replace(url, "/?t=", "/block?t=", 1)
	return openBlockerWindow(blockURL, a)
}
