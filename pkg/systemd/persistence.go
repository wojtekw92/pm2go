package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (m *Manager) savedTargetName() string {
	return m.prefix + "saved.target"
}

func (m *Manager) savedTargetPath() string {
	return filepath.Join(m.getServiceDir(), m.savedTargetName())
}

func (m *Manager) savedWantsDir() string {
	return filepath.Join(m.getServiceDir(), m.savedTargetName()+".wants")
}

func (m *Manager) bootTargetName() string {
	if m.userMode {
		return "default.target"
	}
	return "multi-user.target"
}

func (m *Manager) savedTargetContent() string {
	return fmt.Sprintf(`[Unit]
Description=PM2go saved process set
After=network.target

[Install]
WantedBy=%s
`, m.bootTargetName())
}

func (m *Manager) replaceSavedServices(online, services []string) error {
	desired := make(map[string]bool, len(online))
	for _, serviceName := range online {
		if err := m.validateManagedService(serviceName); err != nil {
			return err
		}
		desired[serviceName+".service"] = true
	}
	existing, err := os.ReadDir(m.savedWantsDir())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read saved membership: %w", err)
	}
	// Validate existing links before changing anything; unrelated files are not ours.
	for _, entry := range existing {
		if err := m.validateSavedLink(entry); err != nil {
			return err
		}
	}

	if err := atomicWriteFile(m.savedTargetPath(), []byte(m.savedTargetContent()), 0644); err != nil {
		return fmt.Errorf("failed to write saved target: %v", err)
	}
	if err := os.MkdirAll(m.savedWantsDir(), 0755); err != nil {
		return err
	}
	bootLink := filepath.Join(m.getServiceDir(), m.bootTargetName()+".wants", m.savedTargetName())
	if err := ensureSymlink(bootLink, filepath.Join("..", m.savedTargetName())); err != nil {
		return err
	}
	for _, service := range online {
		name := service + ".service"
		if err := ensureSymlink(filepath.Join(m.savedWantsDir(), name), filepath.Join("..", name)); err != nil {
			return fmt.Errorf("saved membership update incomplete: %w", err)
		}
	}
	for _, entry := range existing {
		if !desired[entry.Name()] {
			if err := removeFileIfExists(filepath.Join(m.savedWantsDir(), entry.Name())); err != nil {
				return fmt.Errorf("saved membership update incomplete: %w", err)
			}
		}
	}
	if err := m.removeLegacyBootLinks(services); err != nil {
		return fmt.Errorf("saved membership updated, but legacy boot link cleanup failed: %w", err)
	}
	if err := m.systemdReload(); err != nil {
		return fmt.Errorf("saved membership updated on disk, but systemd reload failed: %w", err)
	}
	return nil
}

func (m *Manager) savedServices() ([]string, error) {
	entries, err := os.ReadDir(m.savedWantsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no saved process list found; run 'pm2go save' first")
		}
		return nil, fmt.Errorf("failed to read saved process set: %v", err)
	}

	services := make([]string, 0, len(entries))
	var errors []string
	for _, entry := range entries {
		if err := m.validateSavedLink(entry); err != nil {
			errors = append(errors, err.Error())
			continue
		}
		serviceName := strings.TrimSuffix(entry.Name(), ".service")
		if err := m.validateManagedService(serviceName); err != nil {
			errors = append(errors, err.Error())
			continue
		}
		services = append(services, serviceName)
	}
	if len(errors) > 0 {
		return services, fmt.Errorf("invalid saved process members: %s", strings.Join(errors, "; "))
	}
	return services, nil
}

func (m *Manager) validateSavedLink(entry os.DirEntry) error {
	name := entry.Name()
	if entry.Type()&os.ModeSymlink == 0 || !strings.HasPrefix(name, m.prefix) || !strings.HasSuffix(name, ".service") {
		return fmt.Errorf("saved process set contains invalid entry %s", name)
	}
	target, err := os.Readlink(filepath.Join(m.savedWantsDir(), name))
	if err != nil {
		return err
	}
	if target != filepath.Join("..", name) {
		return fmt.Errorf("saved link %s has invalid target %s", name, target)
	}
	return nil
}

func (m *Manager) validateManagedService(serviceName string) error {
	if filepath.Base(serviceName) != serviceName || !strings.HasPrefix(serviceName, m.prefix) {
		return fmt.Errorf("invalid managed service name %q", serviceName)
	}
	if _, _, err := m.parseServiceName(serviceName); err != nil {
		return fmt.Errorf("invalid managed service name %q: %v", serviceName, err)
	}
	path := filepath.Join(m.getServiceDir(), serviceName+".service")
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("saved service %s is unavailable: %v", serviceName, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("saved service %s is not a regular unit file", serviceName)
	}
	return nil
}

func (m *Manager) removeServiceLinks(service string) error {
	if err := removeFileIfExists(filepath.Join(m.savedWantsDir(), service+".service")); err != nil {
		return err
	}
	return m.removeLegacyBootLinks([]string{service})
}

func (m *Manager) removeLegacyBootLinks(services []string) error {
	for _, service := range services {
		for _, target := range []string{"default.target", "multi-user.target"} {
			if m.userMode && target == "multi-user.target" {
				continue
			}
			path := filepath.Join(m.getServiceDir(), target+".wants", service+".service")
			if err := removeFileIfExists(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureSymlink(path, target string) error {
	if existing, err := os.Readlink(path); err == nil {
		if existing != target {
			return fmt.Errorf("link %s points to %s instead of %s", path, existing, target)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.Symlink(target, path)
}

func removeFileIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
