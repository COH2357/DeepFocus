//go:build windows

package main

import (
	"flag"
	"focusd/gui"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--block" {
		fs := flag.NewFlagSet("deepfocus_gui --block", flag.ContinueOnError)
		kind := fs.String("kind", "app", "app or site")
		target := fs.String("target", "", "limit key or site class")
		label := fs.String("label", "", "display label")
		limit := fs.Int("limit", 0, "limit in minutes")
		used := fs.Int("used", 0, "used seconds")
		hwnd := fs.Uint64("hwnd", 0, "window to restore after bypass")
		pid := fs.Uint64("pid", 0, "process to watch for an app limit")
		if err := fs.Parse(os.Args[2:]); err != nil {
			gui.ShowFatal(err)
			return
		}
		if err := gui.RunBlocker(gui.BlockOptions{
			Kind:         *kind,
			Target:       *target,
			Label:        *label,
			LimitMinutes: *limit,
			UsedSeconds:  *used,
			HWND:         uintptr(*hwnd),
			PID:          uint32(*pid),
		}); err != nil {
			gui.ShowFatal(err)
		}
		return
	}
	if err := gui.Run(); err != nil {
		gui.ShowFatal(err)
	}
}
