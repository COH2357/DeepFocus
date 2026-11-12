//go:build !windows

package gui

import (
	"fmt"
	"os/exec"
	"runtime"
)

// ShowFatal reports a startup failure.
func ShowFatal(err error) {
	fmt.Println("DeepFocus could not start:", err)
}

// setTitleBarTheme is a no-op outside Windows.
func setTitleBarTheme(dark bool) {}

// openWindow falls back to the default browser on platforms without the
// WebView2 backend.
func openWindow(url string, a *app) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Println("Open this address in a browser:", url)
	}
	return nil
}
