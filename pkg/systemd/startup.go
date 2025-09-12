package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// IsStartupConfigured checks if startup configuration is already done
func (m *Manager) IsStartupConfigured() bool {
	if !m.userMode {
		return true // System services don't need startup config
	}
	lingeringOk := m.checkLingering() == "yes"
	userServiceOk := m.checkUserService() == "enabled"
	return lingeringOk && userServiceOk
}

// GetStartupStatus returns the current startup configuration status
func (m *Manager) GetStartupStatus() (lingering, userService string) {
	return m.checkLingering(), m.checkUserService()
}

// ConfigureStartup configures the complete startup environment
func (m *Manager) ConfigureStartup() error {
	if !m.userMode {
		return fmt.Errorf("running as root - system services don't need startup configuration")
	}

	// Step 1: Enable lingering
	if err := m.enableLingering(); err != nil {
		return fmt.Errorf("lingering setup failed: %v", err)
	}

	// Step 2: Enable user systemd service
	if err := m.enableUserSystemdService(); err != nil {
		return fmt.Errorf("user systemd service setup failed: %v", err)
	}

	// Step 3: Reload systemd
	if err := m.reloadUserSystemd(); err != nil {
		return fmt.Errorf("systemd reload failed: %v", err)
	}

	return nil
}

// GetStartupCommands returns the manual commands needed for startup configuration
func (m *Manager) GetStartupCommands() []string {
	if !m.userMode {
		return []string{}
	}

	username := m.getCurrentUser()
	uid := os.Getuid()

	return []string{
		fmt.Sprintf("sudo loginctl enable-linger %s", username),
		fmt.Sprintf("sudo systemctl enable user@%d.service", uid),
		"systemctl --user daemon-reload",
	}
}

// enableLingering enables systemd user lingering so services persist after logout
func (m *Manager) enableLingering() error {
	if !m.userMode {
		return nil // Not needed for system services
	}

	// Check if lingering is already enabled
	username := m.getCurrentUser()
	_, err := os.Stat(fmt.Sprintf("/var/lib/systemd/linger/%s", username))
	if err == nil {
		return nil // Already enabled
	}

	// Enable lingering
	cmd := exec.Command("sudo", "loginctl", "enable-linger", username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable lingering: %v", err)
	}

	return nil
}

// enableUserSystemdService enables the user@UID.service for early boot startup
func (m *Manager) enableUserSystemdService() error {
	if !m.userMode {
		return nil // Not needed for system services
	}

	uid := os.Getuid()
	serviceName := fmt.Sprintf("user@%d.service", uid)

	// Check if already enabled
	cmd := exec.Command("systemctl", "is-enabled", serviceName)
	output, _ := cmd.Output()
	if strings.TrimSpace(string(output)) == "enabled" {
		return nil // Already enabled
	}

	// Enable the service
	cmd = exec.Command("sudo", "systemctl", "enable", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable user systemd service: %v", err)
	}

	return nil
}

// reloadUserSystemd reloads the user systemd daemon
func (m *Manager) reloadUserSystemd() error {
	if !m.userMode {
		return nil // Not needed for system services
	}

	cmd := exec.Command("systemctl", "--user", "daemon-reload")
	return cmd.Run()
}

// checkLingering checks if user lingering is enabled
func (m *Manager) checkLingering() string {
	username := m.getCurrentUser()
	_, err := os.Stat(fmt.Sprintf("/var/lib/systemd/linger/%s", username))
	if err == nil {
		return "yes"
	}
	return "no"
}

// checkUserService checks if user@UID.service is enabled
func (m *Manager) checkUserService() string {
	uid := os.Getuid()
	cmd := exec.Command("systemctl", "is-enabled", fmt.Sprintf("user@%d.service", uid))
	output, err := cmd.Output()
	if err != nil {
		return "disabled"
	}
	return strings.TrimSpace(string(output))
}