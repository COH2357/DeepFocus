//go:build windows

package main

import (
	"focusd/gui"
)

func main() {
	if err := gui.Run(); err != nil {
		gui.ShowFatal(err)
	}
}
