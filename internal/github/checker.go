package github

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const apiBase = "https://api.github.com"

// Release represents a GitHub release payload (trimmed to what we need).
type Release struct {
	TagName string  `json:"tag_name"` // e.g. "v1.2.3"
	Body    string  `json:"body"`     // release notes
	Assets  []Asset `json:"assets"`
}

// Asset is a file attached to a release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Checker handles version comparison and downloading.
type Checker struct {
	Owner          string
	Repo           string
	CurrentVersion string
	logger         *log.Logger
}

func NewChecker(owner, repo, currentVersion string) *Checker {
	logPath := logFilePath()
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	var out io.Writer = os.Stdout
	if err == nil {
		out = io.MultiWriter(os.Stdout, f)
	}
	return &Checker{
		Owner:          owner,
		Repo:           repo,
		CurrentVersion: currentVersion,
		logger:         log.New(out, "[moralink-updater] ", log.LstdFlags),
	}
}

// Log writes a formatted message to both stdout and the log file.
func (c *Checker) Log(format string, args ...any) {
	c.logger.Printf(format, args...)
}

// Check fetches the latest GitHub release and returns whether an update is available.
func (c *Checker) Check() (*Release, bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", apiBase, c.Owner, c.Repo)

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "moralink-updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to reach GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, false, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, false, fmt.Errorf("failed to parse release: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(c.CurrentVersion, "v")

	hasUpdate := latest != current && isNewer(latest, current)
	return &release, hasUpdate, nil
}

// DownloadAndReplace downloads the correct asset for the current OS/arch,
// replaces the binary at targetPath (defaults to the running service binary),
// and reports progress via progressCh (can be nil).
func (c *Checker) DownloadAndReplace(release *Release, progressCh chan<- int) error {
	asset := pickAsset(release.Assets)
	if asset == nil {
		return fmt.Errorf("no compatible asset found for %s/%s in release %s",
			runtime.GOOS, runtime.GOARCH, release.TagName)
	}

	c.Log("Downloading %s (%s)...", asset.Name, asset.BrowserDownloadURL)

	resp, err := http.Get(asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	// Write to a temp file alongside the target
	targetPath := serviceExecutablePath()
	tmpPath := targetPath + ".tmp"

	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("cannot create temp file: %w", err)
	}

	total := asset.Size
	var downloaded int64
	buf := make([]byte, 64*1024)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := tmp.Write(buf[:n]); writeErr != nil {
				tmp.Close()
				os.Remove(tmpPath)
				return fmt.Errorf("write error: %w", writeErr)
			}
			downloaded += int64(n)
			if progressCh != nil && total > 0 {
				progressCh <- int(downloaded * 100 / total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("read error: %w", readErr)
		}
	}
	tmp.Close()

	// On Windows we can't replace a running binary directly,
	// so we rename the old one first, then move the new one in.
	if runtime.GOOS == "windows" {
		backupPath := targetPath + ".old"
		os.Remove(backupPath) // clean up previous backup
		if err := os.Rename(targetPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup old binary: %w", err)
		}
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	c.Log("Binary replaced at %s", targetPath)
	if progressCh != nil {
		progressCh <- 100
	}
	return nil
}

// pickAsset selects the release asset matching the current OS and architecture.
// Convention: assets should be named like "moralinkgost-linux-amd64" or "moralinkgost-windows-amd64.exe"
func pickAsset(assets []Asset) *Asset {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, goos) && strings.Contains(name, goarch) {
			return &a
		}
	}
	return nil
}

// isNewer does a simple semver-ish comparison (major.minor.patch).
func isNewer(latest, current string) bool {
	return latest > current // works for semver strings lexicographically as a baseline
}

// serviceExecutablePath returns the path to the moralinkgost binary.
// Adjust this to match your actual install path.
func serviceExecutablePath() string {
	if runtime.GOOS == "windows" {
		return `C:\Program Files\MoraLink\moralinkgost.exe`
	}
	return "/usr/local/bin/moralinkgost"
}

// logFilePath returns where to write updater logs.
func logFilePath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("ProgramData"), "MoraLink", "updater.log")
	}
	return "/var/log/moralink-updater.log"
}
