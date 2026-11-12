package cli

import (
	"fmt"
	"focusd/system"
	"os"
)

func printHelp() {
	fmt.Println()
	fmt.Println("deepfocus - Privacy-first digital wellbeing tracker")
	fmt.Printf("Version %s\n", system.Version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  deepfocus                    Open the desktop app")
	fmt.Println("  deepfocus gui                Open the desktop app")
	fmt.Println("  deepfocus tui                Open the terminal interface")
	fmt.Println("  deepfocus start              Start tracking (background)")
	fmt.Println("  deepfocus stop               Stop tracking")
	fmt.Println()
	fmt.Println("Setup:")
	fmt.Println("  deepfocus update             Open the releases page (manual)")
	fmt.Println()
	fmt.Println("Viewing Data:")
	fmt.Println("  deepfocus status    (s)      Show tracking status")
	fmt.Println("  deepfocus stats     (st)     Detailed usage breakdown")
	fmt.Println()
	fmt.Println("Tracking Control:")
	fmt.Println("  deepfocus pause     (p)      Pause tracking")
	fmt.Println("  deepfocus resume    (r)      Resume tracking")
	fmt.Println("  deepfocus focus [min]        Start Pomodoro timer")
	fmt.Println("  deepfocus limit [app] [min]  Set daily app limit")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  deepfocus retention (ret)    Show/set retention days")
	fmt.Println("  deepfocus autostart (auto)   Manage auto-start")
	fmt.Println("  deepfocus path               Manage PATH integration")
	fmt.Println("  deepfocus browser            Manage custom browsers")
	fmt.Println()
	fmt.Println("Other:")
	fmt.Println("  deepfocus help      (h)      Show this help message")
	fmt.Println("  deepfocus version   (-v)     Show version")
	fmt.Println()
}

func printVersion() {
	fmt.Printf("deepfocus version %s\n", system.Version)
}

func Run(args []string) {
	if len(args) < 2 {
		runDesktop()
		return
	}

	command := args[1]

	switch command {
	case "gui", "desktop":
		runDesktop()
	case "tui":
		runInteractiveMenu()
	case "update":
		runUpdate()
	case "focus":
		runFocus(args)
	case "stop-timer":
		runStopTimer()
	case "limit":
		runLimits(args)
	case "start":
		runStart()
	case "stop":
		runStop()
	case "status", "s":
		runStatus()
	case "stats", "st":
		runStats()
	case "pause", "p":
		runPause()
	case "resume", "r":
		runResume()
	case "retention", "ret":
		handleRetention(args)
	case "autostart", "auto":
		handleAutostart(args)
	case "path":
		handlePath(args)
	case "browser":
		handleBrowsersCommand(args)
	case "help", "-h", "--help", "h":
		printHelp()
	case "version", "-v", "--version":
		printVersion()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Run 'deepfocus help' for usage information.")
		os.Exit(1)
	}
}

func handleRetention(args []string) {
	if len(args) < 3 {
		runRetentionStatus()
		return
	}
	switch args[2] {
	case "status":
		runRetentionStatus()
	case "set":
		if len(args) < 4 {
			fmt.Println("Usage: deepfocus retention set <days>")
			os.Exit(1)
		}
		runRetentionSet(args[3])
	case "reset":
		runRetentionReset()
	default:
		fmt.Printf("Unknown retention command: %s\n", args[2])
		os.Exit(1)
	}
}

func handleAutostart(args []string) {
	if len(args) < 3 {
		runAutostartStatus()
		return
	}
	switch args[2] {
	case "enable":
		runAutostartEnable()
	case "disable":
		runAutostartDisable()
	case "status":
		runAutostartStatus()
	default:
		fmt.Printf("Unknown autostart command: %s\n", args[2])
		os.Exit(1)
	}
}

func handlePath(args []string) {
	if len(args) < 3 {
		runPathStatus()
		return
	}
	switch args[2] {
	case "enable":
		runPathEnable()
	case "disable":
		runPathDisable()
	case "status":
		runPathStatus()
	default:
		fmt.Printf("Unknown path command: %s\n", args[2])
		os.Exit(1)
	}
}
