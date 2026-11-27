//go:build windows

package main

import (
	"fmt"
	"os"

	"focusd/nativehost"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--install" {
		if err := nativehost.Install(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := nativehost.Run(); err != nil {
		os.Exit(1)
	}
}
