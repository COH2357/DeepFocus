package cli

import (
	"fmt"
	"focusd/system"
	"focusd/ui"
)

// runUpdate intentionally does not download or execute anything.
//
// Installing an arbitrary executable fetched from a remote API is a supply
// chain risk: a compromised account, a bad release, or an incompatible build
// can overwrite the running installation without the user inspecting it. The
// update check therefore only points at the project's release page; the user
// reviews the release notes and runs the installer manually.
func runUpdate() {
	ui.PrintHeader()
	fmt.Printf("Current Version: %s\n", system.Version)
	fmt.Println()
	ui.PrintInfo("Automatic updates are disabled in this build.")
	fmt.Println("DeepFocus never downloads or runs an installer on its own.")
	fmt.Printf("Review the release notes and download manually from:\n  https://github.com/%s/%s/releases\n", system.RepoOwner, system.RepoName)
	fmt.Println()
	fmt.Println("Third-party open-source components are listed in THIRD_PARTY_NOTICES.md.")
}
