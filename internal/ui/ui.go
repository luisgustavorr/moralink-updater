package ui

import (
	"fmt"
	"strings"
	"time"

	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/luisgustavorr/moralink-updater/internal/github"
	"github.com/luisgustavorr/moralink-updater/internal/service"
)

// Run opens the updater window. This is the entry point for GUI mode.
func Run(currentVersion string, checker *github.Checker, svc *service.Manager) {
	a := app.New()
	a.Settings().SetTheme(theme.DarkTheme())

	w := a.NewWindow("MoraLink Updater")
	w.Resize(fyne.NewSize(480, 360))
	w.SetFixedSize(true)
	w.CenterOnScreen()

	content := buildUI(currentVersion, checker, svc, w)
	w.SetContent(content)
	w.ShowAndRun()
}

func buildUI(currentVersion string, checker *github.Checker, svc *service.Manager, w fyne.Window) fyne.CanvasObject {
	// ── Header ────────────────────────────────────────────────────────────────
	title := canvas.NewText("MoraLink Updater", color.White)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	subtitle := canvas.NewText("Manage your MoraLink service updates", color.NRGBA{R: 180, G: 180, B: 180, A: 255})
	subtitle.TextSize = 13
	subtitle.Alignment = fyne.TextAlignCenter

	// ── Version cards ─────────────────────────────────────────────────────────
	currentLabel := widget.NewLabel(fmt.Sprintf("Installed version:  v%s", strings.TrimPrefix(currentVersion, "v")))
	latestLabel := widget.NewLabel("Latest version:  checking...")
	releaseNotesLabel := widget.NewLabel("")
	releaseNotesLabel.Wrapping = fyne.TextWrapWord

	// ── Service status ────────────────────────────────────────────────────────
	statusDot := canvas.NewCircle(color.NRGBA{R: 80, G: 200, B: 80, A: 255})
	statusDot.SetMinSize(fyne.NewSize(12, 12))
	statusText := widget.NewLabel("Service: checking...")
	statusRow := container.NewHBox(statusDot, statusText)

	// ── Progress bar ──────────────────────────────────────────────────────────
	progressBar := widget.NewProgressBar()
	progressBar.Hide()
	progressInfo := widget.NewLabel("")
	progressInfo.Alignment = fyne.TextAlignCenter
	progressInfo.Hide()

	// ── Buttons ───────────────────────────────────────────────────────────────
	updateBtn := widget.NewButton("Update Now", nil)
	updateBtn.Importance = widget.HighImportance
	updateBtn.Disable()

	checkBtn := widget.NewButton("Check Again", nil)

	// ── State helpers ─────────────────────────────────────────────────────────
	var latestRelease *github.Release

	setStatus := func(running bool) {
		if running {
			statusDot.FillColor = color.NRGBA{R: 80, G: 200, B: 80, A: 255}
			statusText.SetText("Service: running")
		} else {
			statusDot.FillColor = color.NRGBA{R: 220, G: 60, B: 60, A: 255}
			statusText.SetText("Service: stopped")
		}
		statusDot.Refresh()
	}

	doCheck := func() {
		checkBtn.Disable()
		latestLabel.SetText("Latest version:  checking...")
		updateBtn.Disable()

		go func() {
			// Check service status
			out, _ := svc.Status()
			running := strings.Contains(out, "active") || strings.Contains(out, "RUNNING")
			setStatus(running)

			// Check for updates
			release, hasUpdate, err := checker.Check()
			if err != nil {
				latestLabel.SetText(fmt.Sprintf("Error: %v", err))
				checkBtn.Enable()
				return
			}

			latestRelease = release
			latestLabel.SetText(fmt.Sprintf("Latest version:  %s", release.TagName))

			if hasUpdate {
				updateBtn.Enable()
				notes := release.Body
				if len(notes) > 200 {
					notes = notes[:200] + "..."
				}
				releaseNotesLabel.SetText("What's new:\n" + notes)
			} else {
				releaseNotesLabel.SetText("✓  You are up to date.")
			}
			checkBtn.Enable()
		}()
	}

	// ── Button actions ────────────────────────────────────────────────────────
	checkBtn.OnTapped = func() {
		doCheck()
	}

	updateBtn.OnTapped = func() {
		if latestRelease == nil {
			return
		}

		updateBtn.Disable()
		checkBtn.Disable()
		progressBar.Show()
		progressBar.SetValue(0)
		progressInfo.Show()
		progressInfo.SetText("Stopping service...")

		go func() {
			// Stop service
			if err := svc.Stop(); err != nil {
				progressInfo.SetText(fmt.Sprintf("Failed to stop service: %v", err))
				updateBtn.Enable()
				checkBtn.Enable()
				return
			}

			progressInfo.SetText("Downloading update...")
			progressCh := make(chan int, 50)

			// Update progress bar from channel
			go func() {
				for p := range progressCh {
					progressBar.SetValue(float64(p) / 100.0)
					progressInfo.SetText(fmt.Sprintf("Downloading... %d%%", p))
				}
			}()

			// Download and replace
			if err := checker.DownloadAndReplace(latestRelease, progressCh); err != nil {
				close(progressCh)
				progressInfo.SetText(fmt.Sprintf("Download failed: %v", err))
				// Try to restart service anyway
				svc.Start()
				setStatus(true)
				checkBtn.Enable()
				return
			}
			close(progressCh)

			// Restart service
			progressInfo.SetText("Restarting service...")
			time.Sleep(500 * time.Millisecond)

			if err := svc.Start(); err != nil {
				progressInfo.SetText(fmt.Sprintf("Warning: service restart failed: %v", err))
			} else {
				progressInfo.SetText("✓  Update complete! Service is running.")
				setStatus(true)
			}

			progressBar.SetValue(1)
			checkBtn.Enable()
			// Re-check so version labels refresh
			doCheck()
		}()
	}

	// ── Layout ────────────────────────────────────────────────────────────────
	versionCard := container.NewVBox(
		currentLabel,
		latestLabel,
		widget.NewSeparator(),
		releaseNotesLabel,
	)

	buttons := container.NewHBox(checkBtn, updateBtn)

	layout := container.NewVBox(
		container.NewPadded(title),
		subtitle,
		widget.NewSeparator(),
		container.NewPadded(versionCard),
		container.NewPadded(statusRow),
		widget.NewSeparator(),
		container.NewPadded(progressBar),
		container.NewCenter(progressInfo),
		container.NewCenter(buttons),
	)

	// Initial check on open
	go doCheck()

	return layout
}
