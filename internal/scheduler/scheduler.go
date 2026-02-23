package scheduler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const taskName = "MoraLinkUpdater"

// Install registers the updater to run daily at 9am via cron (Linux) or Task Scheduler (Windows).
func Install() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	exe, _ = filepath.Abs(exe)

	switch runtime.GOOS {
	case "linux":
		return installCron(exe)
	case "windows":
		return installTaskScheduler(exe)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Uninstall removes the scheduled task.
func Uninstall() error {
	switch runtime.GOOS {
	case "linux":
		return uninstallCron()
	case "windows":
		return uninstallTaskScheduler()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// ── Linux / cron ──────────────────────────────────────────────────────────────

const cronMarker = "# moralink-updater"

func installCron(exe string) error {
	// Read existing crontab
	out, _ := exec.Command("crontab", "-l").Output()
	existing := string(out)

	// Remove any old entry first
	existing = removeCronEntry(existing)

	// Add new entry: every day at 09:00
	entry := fmt.Sprintf("0 9 * * * %s --headless %s\n", exe, cronMarker)
	newCrontab := existing + entry

	// Write back via "crontab -"
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCrontab)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("crontab write failed: %w — %s", err, string(out))
	}
	return nil
}

func uninstallCron() error {
	out, _ := exec.Command("crontab", "-l").Output()
	cleaned := removeCronEntry(string(out))

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(cleaned)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("crontab write failed: %w — %s", err, string(out))
	}
	return nil
}

func removeCronEntry(crontab string) string {
	var lines []string
	for _, line := range strings.Split(crontab, "\n") {
		if !strings.Contains(line, cronMarker) {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// ── Windows / Task Scheduler ──────────────────────────────────────────────────

func installTaskScheduler(exe string) error {
	// Delete existing task silently (ignore error if it doesn't exist)
	exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()

	// Create a daily task at 09:00 running as SYSTEM
	args := []string{
		"/Create",
		"/TN", taskName,
		"/TR", fmt.Sprintf(`"%s" --headless`, exe),
		"/SC", "DAILY",
		"/ST", "09:00",
		"/RU", "SYSTEM",    // run even when no user is logged in
		"/RL", "HIGHEST",   // run with elevated privileges (needed to restart service)
		"/F",               // force overwrite
	}
	cmd := exec.Command("schtasks", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks create failed: %w — %s", err, string(out))
	}
	return nil
}

func uninstallTaskScheduler() error {
	cmd := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks delete failed: %w — %s", err, string(out))
	}
	return nil
}
