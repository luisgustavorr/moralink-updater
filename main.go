package main

import (
	"flag"
	"fmt"
	"moralinkgost-updater/internal/github"
	"moralinkgost-updater/internal/scheduler"
	"moralinkgost-updater/internal/service"
	"moralinkgost-updater/internal/ui"
	"os"
)

// Version is injected at build time via -ldflags
// Example: go build -ldflags "-X main.Version=1.2.3"
var Version = "0.0.1"

func main() {
	headless := flag.Bool("headless", false, "Run without UI (used by scheduler)")
	install := flag.Bool("install-scheduler", false, "Register updater in cron/Task Scheduler and exit")
	uninstall := flag.Bool("uninstall-scheduler", false, "Remove updater from cron/Task Scheduler and exit")
	flag.Parse()

	// Handle scheduler registration
	if *install {
		if err := scheduler.Install(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install scheduler: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Scheduler installed successfully.")
		return
	}
	if *uninstall {
		if err := scheduler.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to uninstall scheduler: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Scheduler removed successfully.")
		return
	}

	checker := github.NewChecker(
		"luisgustavorr",
		"moralink-updater",
		Version,
	)

	svcManager := service.NewManager("moralinkgost")

	// Headless mode: check and update silently (triggered by scheduler)
	if *headless {
		runHeadless(checker, svcManager)
		return
	}

	// GUI mode: open Fyne window
	ui.Run(Version, checker, svcManager)
}

func runHeadless(checker *github.Checker, svc *service.Manager) {
	release, hasUpdate, err := checker.Check()
	if err != nil {
		checker.Log("Headless check failed: %v", err)
		return
	}
	if !hasUpdate {
		checker.Log("No update available (current: %s)", checker.CurrentVersion)
		return
	}
	checker.Log("Update found: %s → applying...", release.TagName)

	if err := svc.Stop(); err != nil {
		checker.Log("Failed to stop service: %v", err)
		return
	}

	if err := checker.DownloadAndReplace(release, nil); err != nil {
		checker.Log("Download failed: %v", err)
		svc.Start() // try to restart even if update failed
		return
	}

	if err := svc.Start(); err != nil {
		checker.Log("Failed to restart service: %v", err)
		return
	}

	checker.Log("Update to %s completed successfully.", release.TagName)
}
