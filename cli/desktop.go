package cli

import (
	"fmt"
	"focusd/gui"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// runDesktop opens the graphical app. The standalone GUI binary is preferred
// when it sits next to the CLI so the console window can close immediately.
// Terminals without a desktop session (SSH, other platforms) keep the TUI,
// which is still available as `deepfocus tui`.
func runDesktop() {
	if runtime.GOOS != "windows" {
		runInteractiveMenu()
		return
	}
	if spawnGUI() {
		return
	}
	if err := gui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open the desktop app: %v\n", err)
		fmt.Fprintln(os.Stderr, "Run 'deepfocus tui' for the terminal interface.")
		os.Exit(1)
	}
}

func spawnGUI() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	if filepath.Base(exe) != "deepfocus.exe" {
		return false
	}
	guiExe := filepath.Join(filepath.Dir(exe), "deepfocus_gui.exe")
	if info, err := os.Stat(guiExe); err != nil || info.IsDir() {
		return false
	}
	cmd := exec.Command(guiExe)
	cmd.Dir = filepath.Dir(guiExe)
	if err := cmd.Start(); err != nil {
		return false
	}
	_ = cmd.Process.Release()
	return true
}
