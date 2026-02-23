package service

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// Manager controls the moralinkgost system service.
type Manager struct {
	Name string
}

func NewManager(serviceName string) *Manager {
	return &Manager{Name: serviceName}
}

// Stop halts the service and waits briefly for it to settle.
func (m *Manager) Stop() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("systemctl", "stop", m.Name)
	case "windows":
		cmd = exec.Command("sc", "stop", m.Name)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stop failed: %w — %s", err, string(out))
	}

	// Give the OS a moment to fully release the binary file
	time.Sleep(2 * time.Second)
	return nil
}

// Start launches the service.
func (m *Manager) Start() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("systemctl", "start", m.Name)
	case "windows":
		cmd = exec.Command("sc", "start", m.Name)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("start failed: %w — %s", err, string(out))
	}
	return nil
}

// Status returns a human-readable service status string.
func (m *Manager) Status() (string, error) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("systemctl", "is-active", m.Name)
	case "windows":
		cmd = exec.Command("sc", "query", m.Name)
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	out, _ := cmd.Output() // non-zero exit is normal when stopped
	return string(out), nil
}
