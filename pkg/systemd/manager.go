package systemd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Manager handles systemd operations for PM2-style process management
type Manager struct {
	userMode   bool
	prefix     string // prefix for service names to avoid conflicts
	serviceDir string
	initErr    error
}

// NewManager creates a new systemd manager instance
func NewManager() *Manager {
	prefix := os.Getenv("PM2GO_SERVICE_PREFIX")
	if prefix == "" {
		prefix = "pm2-"
	}
	manager := &Manager{userMode: os.Getuid() != 0, prefix: prefix}
	if err := validateServicePrefix(prefix); err != nil {
		manager.initErr = err
	}
	if manager.userMode {
		if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
			if !filepath.IsAbs(configHome) {
				manager.initErr = fmt.Errorf("XDG_CONFIG_HOME must be an absolute path")
			} else {
				manager.serviceDir = filepath.Join(configHome, "systemd/user")
			}
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				manager.initErr = fmt.Errorf("failed to determine user home directory: %v", err)
			} else if !filepath.IsAbs(home) {
				manager.initErr = fmt.Errorf("user home directory must be an absolute path")
			} else {
				manager.serviceDir = filepath.Join(home, ".config/systemd/user")
			}
		}
	} else {
		manager.serviceDir = "/etc/systemd/system"
	}
	return manager
}

func validateServicePrefix(prefix string) error {
	if prefix == "" || strings.Contains(prefix, "..") {
		return fmt.Errorf("invalid PM2GO_SERVICE_PREFIX %q", prefix)
	}
	for _, r := range prefix {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("_.:-", r) {
			continue
		}
		return fmt.Errorf("invalid PM2GO_SERVICE_PREFIX %q", prefix)
	}
	return nil
}

// serviceNameWithID returns the systemd service name with ID for an app
func (m *Manager) serviceNameWithID(id int, appName string) string {
	return fmt.Sprintf("%s%d-%s", m.prefix, id, escapeUnitNamePart(appName))
}

// parseServiceName parses service name and returns ID and app name
func (m *Manager) parseServiceName(serviceName string) (int, string, error) {
	if !strings.HasPrefix(serviceName, m.prefix) {
		return 0, "", fmt.Errorf("invalid service name format")
	}

	// Remove prefix: pm2-{id}-{name}.service -> {id}-{name}.service
	remaining := strings.TrimPrefix(serviceName, m.prefix)
	remaining = strings.TrimSuffix(remaining, ".service")

	// Find first dash to separate ID from name
	dashIndex := strings.Index(remaining, "-")
	if dashIndex == -1 {
		// Old format without ID, return ID 0
		return 0, unescapeUnitNamePart(remaining), nil
	}

	idStr := remaining[:dashIndex]
	appName := unescapeUnitNamePart(remaining[dashIndex+1:])

	id, err := strconv.Atoi(idStr)
	if err != nil {
		// Old format without ID, treat whole thing as name
		return 0, unescapeUnitNamePart(remaining), nil
	}

	return id, appName, nil
}

func escapeUnitNamePart(value string) string {
	var escaped strings.Builder
	for _, b := range []byte(value) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == ':' || b == '.' || b == '-' {
			escaped.WriteByte(b)
			continue
		}
		fmt.Fprintf(&escaped, "\\x%02x", b)
	}
	return escaped.String()
}

func unescapeUnitNamePart(value string) string {
	bytes := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+3 < len(value) && value[i+1] == 'x' {
			if b, err := strconv.ParseUint(value[i+2:i+4], 16, 8); err == nil {
				bytes = append(bytes, byte(b))
				i += 3
				continue
			}
		}
		bytes = append(bytes, value[i])
	}
	return string(bytes)
}

// managedServiceNames discovers definitions without querying runtime state.
func (m *Manager) managedServiceNames() ([]string, error) {
	if m.initErr != nil {
		return nil, m.initErr
	}
	files, err := filepath.Glob(filepath.Join(m.getServiceDir(), m.prefix+"*.service"))
	if err != nil {
		return nil, err
	}
	services := make([]string, 0, len(files))
	for _, file := range files {
		services = append(services, strings.TrimSuffix(filepath.Base(file), ".service"))
	}
	return services, nil
}

// findServiceByIdentifier finds service name by ID or name
func (m *Manager) findServiceByIdentifier(identifier string) (string, error) {
	services, err := m.managedServiceNames()
	if err != nil {
		return "", err
	}
	id, idErr := strconv.Atoi(identifier)
	for _, service := range services {
		serviceID, name, err := m.parseServiceName(service)
		if err != nil {
			continue
		}
		if (idErr == nil && serviceID == id) || (idErr != nil && name == identifier) {
			return service, nil
		}
	}
	return "", fmt.Errorf("process '%s' not found", identifier)
}

// Start creates and starts a systemd service for the given app config
func (m *Manager) Start(config AppConfig) error {
	return m.withMutationLock(func() error {
		if config.Name == "" {
			return fmt.Errorf("application name is required")
		}
		if config.Script == "" {
			return fmt.Errorf("application script is required")
		}
		services, err := m.managedServiceNames()
		if err != nil {
			return err
		}
		nextID := 0
		for _, service := range services {
			id, name, err := m.parseServiceName(service)
			if err != nil {
				return err
			}
			if name == config.Name {
				return fmt.Errorf("process with name '%s' already exists (ID: %d)", name, id)
			}
			if config.ID != 0 && config.ID == id {
				return fmt.Errorf("process with ID %d already exists", config.ID)
			}
			if id >= nextID {
				nextID = id + 1
			}
		}
		if config.ID == 0 {
			config.ID = nextID
		}
		config.CreatedAt = time.Now().UnixMilli()

		if err := m.prepareLogDirectories(config); err != nil {
			return err
		}

		serviceName := m.serviceNameWithID(config.ID, config.Name)
		servicePath := filepath.Join(m.getServiceDir(), serviceName+".service")
		if _, err := os.Lstat(servicePath); err == nil {
			return fmt.Errorf("service file %s already exists", servicePath)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("failed to inspect service file: %v", err)
		}

		if err := atomicWriteFile(servicePath, []byte(m.generateServiceFile(config)), 0600); err != nil {
			return fmt.Errorf("failed to write service file: %v", err)
		}
		if err := m.systemdReload(); err != nil {
			return fmt.Errorf("unit written to %s, but systemd reload failed: %w", servicePath, err)
		}
		if err := m.systemdCommand("start", serviceName); err != nil {
			return fmt.Errorf("failed to start %s; unit retained at %s: %w", config.Name, servicePath, err)
		}
		return nil
	})
}

// Stop stops a systemd service
func (m *Manager) Stop(identifier string) error {
	return m.withMutationLock(func() error {
		if identifier == "all" {
			return m.stopAll()
		}

		serviceName, err := m.findServiceByIdentifier(identifier)
		if err != nil {
			return err
		}
		return m.systemdCommand("stop", serviceName)
	})
}

// stopAll stops all pm2go managed services without deleting their unit files.
func (m *Manager) stopAll() error {
	services, err := m.managedServiceNames()
	if err != nil {
		return err
	}

	var errors []string
	for _, serviceName := range services {
		if err := m.systemdCommand("stop", serviceName); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", serviceName, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to stop services: %s", strings.Join(errors, "; "))
	}
	return nil
}

// Save records the current online process set as native systemd target membership.
func (m *Manager) Save() error {
	return m.withMutationLock(func() error {
		services, err := m.managedServiceNames()
		if err != nil {
			return err
		}

		online := make([]string, 0, len(services))
		for _, service := range services {
			switch m.getServiceStatus(service) {
			case "unknown":
				return fmt.Errorf("cannot save while status for %s is unknown", service)
			case "online":
				online = append(online, service)
			}
		}
		return m.replaceSavedServices(online, services)
	})
}

// Resurrect starts the existing units referenced by the saved systemd target.
func (m *Manager) Resurrect() error {
	return m.withMutationLock(func() error {
		var errors []string
		services, err := m.savedServices()
		if err != nil {
			errors = append(errors, err.Error())
		}
		for _, serviceName := range services {
			if err := m.systemdCommand("start", serviceName); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", serviceName, err))
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("failed to resurrect applications: %s", strings.Join(errors, "; "))
		}
		return nil
	})
}

// Restart restarts an existing service by ID
func (m *Manager) Restart(id int) error {
	return m.withMutationLock(func() error {
		serviceName, err := m.findServiceByIdentifier(strconv.Itoa(id))
		if err != nil {
			return err
		}
		return m.systemdCommand("restart", serviceName)
	})
}

// SendSignal sends a Unix signal to a managed service's main process.
func (m *Manager) SendSignal(identifier string, signal string) error {
	serviceName, err := m.findServiceByIdentifier(identifier)
	if err != nil {
		return err
	}

	return m.systemdCommand("kill", "--kill-who=main", "--signal="+signal, serviceName)
}

// Delete stops and removes a systemd service
func (m *Manager) Delete(identifier string) error {
	return m.withMutationLock(func() error {
		if identifier == "all" {
			return m.deleteAll()
		}

		serviceName, err := m.findServiceByIdentifier(identifier)
		if err != nil {
			return err
		}
		return m.deleteServices([]string{serviceName})
	})
}

// deleteAll removes all pm2go managed services
func (m *Manager) deleteAll() error {
	services, err := m.managedServiceNames()
	if err != nil {
		return err
	}
	return m.deleteServices(services)
}

func (m *Manager) deleteServices(serviceNames []string) error {
	var errors []string
	for _, serviceName := range serviceNames {
		if err := m.systemdCommand("stop", serviceName); err != nil {
			errors = append(errors, fmt.Sprintf("%s stop: %v", serviceName, err))
			continue
		}
		if err := m.removeServiceLinks(serviceName); err != nil {
			errors = append(errors, fmt.Sprintf("%s links: %v", serviceName, err))
			continue
		}
		if err := removeFileIfExists(filepath.Join(m.getServiceDir(), serviceName+".service")); err != nil {
			errors = append(errors, fmt.Sprintf("%s unit: %v", serviceName, err))
		}
	}
	if err := m.systemdReload(); err != nil {
		errors = append(errors, fmt.Sprintf("unit files updated, but systemd reload failed: %v", err))
	}
	if len(errors) > 0 {
		return fmt.Errorf("delete incomplete: %s", strings.Join(errors, "; "))
	}
	return nil
}

// List returns a list of all managed processes
func (m *Manager) List() ([]ProcessInfo, error) {
	services, err := m.managedServiceNames()
	if err != nil {
		return nil, err
	}

	processes := []ProcessInfo{}
	for _, serviceName := range services {
		// Parse ID and name from service name
		id, appName, err := m.parseServiceName(serviceName)
		if err != nil {
			continue // Skip invalid service names
		}

		// Get service status
		status := m.getServiceStatus(serviceName)

		// Get PID for the service (will be 0 if stopped)
		pid := m.getServicePID(serviceName)

		// Get memory usage, CPU, and process start time (will be 0 if stopped)
		memory := m.getServiceMemory(pid)
		cpu := m.getServiceCPU(pid)
		startedAt := m.getServiceStartTime(serviceName)

		// Read service configuration for additional details
		config := m.readServiceConfig(serviceName)
		createdAt := config.CreatedAt
		if createdAt == 0 {
			filePath := filepath.Join(m.getServiceDir(), serviceName+".service")
			if info, err := os.Stat(filePath); err == nil {
				createdAt = info.ModTime().UnixMilli()
			}
		}

		process := ProcessInfo{
			PID:  pid,
			Name: appName,
			PMID: id,
			PM2Env: PM2Env{
				ID:               id,
				Name:             appName,
				ExecMode:         "fork_mode",
				Status:           status,
				PMUptime:         startedAt,
				CreatedAt:        createdAt,
				RestartTime:      0,
				UnstableRestarts: 0,
				Versioning:       nil,
				Node:             PM2Node{Version: "unknown"},
				NodeVersion:      "unknown",
				PMExecPath:       config.Script,
				PMCwd:            config.Cwd,
				PMOutLogPath:     config.OutLogPath,
				PMErrLogPath:     config.ErrLogPath,
				PMPidPath:        config.PidPath,
				Interpreter:      config.Interpreter,
				Args:             config.Args,
				Env:              config.Env,
			},
			Monit: PM2Monit{
				Memory: memory,
				CPU:    cpu,
			},
		}
		processes = append(processes, process)
	}

	return processes, nil
}

// readServiceConfig reads and parses a systemd service file to extract configuration
func (m *Manager) readServiceConfig(serviceName string) ServiceConfig {
	config := ServiceConfig{
		Env: make(map[string]string),
	}

	serviceDir := m.getServiceDir()
	filePath := filepath.Join(serviceDir, serviceName+".service")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return config
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "# PM2Go-Script=") {
			config.Script = decodeMetadataString(strings.TrimPrefix(line, "# PM2Go-Script="))
		} else if strings.HasPrefix(line, "# PM2Go-Interpreter=") {
			config.Interpreter = decodeMetadataString(strings.TrimPrefix(line, "# PM2Go-Interpreter="))
		} else if strings.HasPrefix(line, "# PM2Go-Args=") {
			encoded := strings.TrimPrefix(line, "# PM2Go-Args=")
			if data, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
				_ = json.Unmarshal(data, &config.Args)
			}
		} else if strings.HasPrefix(line, "# PM2Go-CreatedAt=") {
			config.CreatedAt, _ = strconv.ParseInt(strings.TrimPrefix(line, "# PM2Go-CreatedAt="), 10, 64)
		} else if strings.HasPrefix(line, "#") {
			continue
		} else if strings.HasPrefix(line, "ExecStart=") && config.Script == "" {
			execStart := strings.TrimPrefix(line, "ExecStart=")
			// Parse the exec command to extract script and interpreter
			parts := splitCommandLine(execStart)
			if len(parts) > 0 {
				config.Interpreter = parts[0]
				if len(parts) > 1 {
					config.Script = parts[1]
					if len(parts) > 2 {
						config.Args = append(AppArgs(nil), parts[2:]...)
					}
				}
			}
		} else if strings.HasPrefix(line, "WorkingDirectory=") {
			config.Cwd = unquoteSystemdValue(strings.TrimPrefix(line, "WorkingDirectory="))
		} else if strings.HasPrefix(line, "Environment=") {
			envLine := strings.TrimPrefix(line, "Environment=")
			envLine = unquoteSystemdValue(envLine)
			// Parse environment variables
			if strings.Contains(envLine, "=") {
				parts := strings.SplitN(envLine, "=", 2)
				if len(parts) == 2 {
					config.Env[parts[0]] = parts[1]
				}
			}
		} else if strings.HasPrefix(line, "StandardOutput=") {
			outLine := unquoteSystemdValue(strings.TrimPrefix(line, "StandardOutput="))
			if strings.HasPrefix(outLine, "append:") {
				config.OutLogPath = unquoteSystemdValue(strings.TrimPrefix(outLine, "append:"))
			}
		} else if strings.HasPrefix(line, "StandardError=") {
			errLine := unquoteSystemdValue(strings.TrimPrefix(line, "StandardError="))
			if strings.HasPrefix(errLine, "append:") {
				config.ErrLogPath = unquoteSystemdValue(strings.TrimPrefix(errLine, "append:"))
			}
		} else if strings.HasPrefix(line, "Restart=") {
			config.Autorestart = strings.TrimPrefix(line, "Restart=") != "no"
		} else if strings.HasPrefix(line, "RestartSec=") {
			config.RestartDelay = parseSystemdDurationMillis(strings.TrimPrefix(line, "RestartSec="))
		} else if strings.HasPrefix(line, "StartLimitBurst=") {
			if value, err := strconv.Atoi(strings.TrimPrefix(line, "StartLimitBurst=")); err == nil {
				config.MaxRestarts = value
			}
		}
	}

	// Generate default PID path
	config.PidPath = fmt.Sprintf("/tmp/%s.pid", serviceName)

	return config
}

// Flush removes logs for all apps or a specific app
func (m *Manager) Flush(appName string) error {
	if appName == "" {
		return m.flushAllLogs()
	}

	processes, err := m.List()
	if err != nil {
		return err
	}

	for _, process := range processes {
		if process.Name == appName || strconv.Itoa(process.PM2Env.ID) == appName {
			return truncateLogFiles(process.PM2Env.PMOutLogPath, process.PM2Env.PMErrLogPath)
		}
	}

	return fmt.Errorf("process '%s' not found", appName)
}

func (m *Manager) flushAllLogs() error {
	processes, err := m.List()
	if err != nil {
		return err
	}

	var logFiles []string
	for _, process := range processes {
		if process.PM2Env.PMOutLogPath != "" {
			logFiles = append(logFiles, process.PM2Env.PMOutLogPath)
		}
		if process.PM2Env.PMErrLogPath != "" {
			logFiles = append(logFiles, process.PM2Env.PMErrLogPath)
		}
	}

	if len(logFiles) == 0 {
		return nil
	}

	return truncateLogFiles(logFiles...)
}

func truncateLogFiles(paths ...string) error {
	var errors []string
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		if err := file.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to truncate logs: %s", strings.Join(errors, "; "))
	}
	return nil
}

func splitCommandLine(input string) []string {
	var result []string
	var current strings.Builder
	var quote rune
	escaped := false
	tokenStarted := false

	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			tokenStarted = true
			continue
		}

		if r == '\\' {
			escaped = true
			tokenStarted = true
			continue
		}

		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}

		if r == '\'' || r == '"' {
			quote = r
			tokenStarted = true
			continue
		}

		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if tokenStarted {
				result = append(result, current.String())
				current.Reset()
				tokenStarted = false
			}
			continue
		}

		current.WriteRune(r)
		tokenStarted = true
	}

	if escaped {
		current.WriteRune('\\')
	}
	if tokenStarted {
		result = append(result, current.String())
	}

	return result
}

func joinSystemdArgs(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, quoteSystemdValue(arg))
	}
	return strings.Join(quoted, " ")
}

func quoteSystemdValue(value string) string {
	escaped := escapeSystemdSpecifiers(value)
	escaped = strings.ReplaceAll(escaped, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	escaped = strings.ReplaceAll(escaped, "\n", "\\n")
	return "\"" + escaped + "\""
}

func systemdPathValue(value string) string {
	var escaped strings.Builder
	for _, r := range value {
		switch r {
		case '%':
			escaped.WriteString("%%")
		case ' ':
			escaped.WriteString("\\x20")
		case '\t':
			escaped.WriteString("\\t")
		case '\\':
			escaped.WriteString("\\\\")
		default:
			escaped.WriteRune(r)
		}
	}
	return escaped.String()
}

func unquoteSystemdValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	value = strings.ReplaceAll(value, "\\n", "\n")
	value = strings.ReplaceAll(value, "\\\"", "\"")
	value = strings.ReplaceAll(value, "\\\\", "\\")
	value = strings.ReplaceAll(value, "%%", "%")
	return value
}

func escapeSystemdSpecifiers(value string) string {
	return strings.ReplaceAll(value, "%", "%%")
}

func parseSystemdDurationMillis(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if strings.HasSuffix(value, "ms") {
		millis, err := strconv.Atoi(strings.TrimSuffix(value, "ms"))
		if err == nil {
			return millis
		}
	}
	if strings.HasSuffix(value, "s") {
		seconds, err := strconv.ParseFloat(strings.TrimSuffix(value, "s"), 64)
		if err == nil {
			return int(seconds * 1000)
		}
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int(seconds * 1000)
}

func formatRestartSec(restartDelayMillis int) string {
	if restartDelayMillis <= 0 {
		return "0"
	}
	if restartDelayMillis%1000 == 0 {
		return strconv.Itoa(restartDelayMillis / 1000)
	}
	return strconv.Itoa(restartDelayMillis) + "ms"
}

func restartPolicy(config AppConfig) string {
	if config.Autorestart != nil && !*config.Autorestart {
		return "no"
	}
	return "always"
}

func startLimitLines(maxRestarts int) string {
	if maxRestarts <= 0 {
		return ""
	}
	return fmt.Sprintf("StartLimitIntervalSec=60\nStartLimitBurst=%d\n", maxRestarts)
}

func resolveLogPaths(config AppConfig, workingDir string) (string, string) {
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".pm2", "logs")
	os.MkdirAll(logDir, 0755)

	outLog := filepath.Join(logDir, config.Name+"-out.log")
	errLog := filepath.Join(logDir, config.Name+"-error.log")

	if config.LogFile != "" {
		logPath := resolveConfiguredLogPath(config.LogFile, workingDir)
		return logPath, logPath
	}
	if config.OutFile != "" {
		outLog = resolveConfiguredLogPath(config.OutFile, workingDir)
	}
	if config.ErrorFile != "" {
		errLog = resolveConfiguredLogPath(config.ErrorFile, workingDir)
	}

	return outLog, errLog
}

func resolveConfiguredLogPath(path string, workingDir string) string {
	if path == "" {
		return path
	}
	if strings.EqualFold(path, "NULL") {
		return "/dev/null"
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workingDir, path)
}

func systemdLogDirective(path string) string {
	if path == "" || path == "/dev/null" {
		return "null"
	}
	return "append:" + systemdPathValue(path)
}

// generateServiceFile creates the systemd service file content
func (m *Manager) generateServiceFile(config AppConfig) string {
	workingDir := config.Cwd
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}
	if absoluteWorkingDir, err := filepath.Abs(workingDir); err == nil {
		workingDir = absoluteWorkingDir
	}

	outLog, errLog := resolveLogPaths(config, workingDir)
	restart := restartPolicy(config)
	restartSec := formatRestartSec(config.RestartDelay)
	startLimits := startLimitLines(config.MaxRestarts)

	var execArgs []string
	appArgs := configArguments(config)

	if config.Interpreter != "" {
		// Use explicit interpreter: "python3 script.py args"
		interpreterFields := splitCommandLine(config.Interpreter)
		if len(interpreterFields) == 0 {
			interpreterFields = []string{config.Interpreter}
		}
		fullPath, err := exec.LookPath(interpreterFields[0])

		var interpreterPath string
		if err != nil {
			// Fallback to original interpreter if not found in PATH
			interpreterPath = interpreterFields[0]
		} else {
			interpreterPath = fullPath
		}

		execArgs = append(execArgs, interpreterPath)
		execArgs = append(execArgs, interpreterFields[1:]...)
		execArgs = append(execArgs, config.Script)
		if len(appArgs) > 0 {
			execArgs = append(execArgs, appArgs...)
		}
	} else {
		// Auto-detect interpreter or use script directly
		if strings.HasSuffix(config.Script, ".py") {
			pythonPath, err := exec.LookPath("python3")
			if err != nil {
				pythonPath = "python3"
			}
			execArgs = append(execArgs, pythonPath, config.Script)
		} else if strings.HasSuffix(config.Script, ".js") {
			nodePath, err := exec.LookPath("node")
			if err != nil {
				nodePath, err = exec.LookPath("nodejs")
				if err != nil {
					nodePath = "node"
				}
			}
			execArgs = append(execArgs, nodePath, config.Script)
		} else {
			// Use script directly (should have shebang)
			execArgs = append(execArgs, config.Script)
		}

		if len(appArgs) > 0 {
			execArgs = append(execArgs, appArgs...)
		}
	}

	execStart := joinSystemdArgs(execArgs)
	metadata := serviceMetadata(config)

	userLine := ""
	if !m.userMode {
		userLine = "User=" + m.getCurrentUser() + "\n"
	}
	environment := make([]string, 0, len(config.Env))
	for key, value := range config.Env {
		environment = append(environment, "Environment="+quoteSystemdValue(key+"="+value))
	}

	return fmt.Sprintf(`[Unit]
%s
Description=PM2 App: %s
After=network.target
%s

[Service]
Type=simple
%sWorkingDirectory=%s
ExecStart=%s
Restart=%s
RestartSec=%s
StandardOutput=%s
StandardError=%s
%s

[Install]
WantedBy=%s
`, metadata, config.Name, startLimits, userLine, systemdPathValue(workingDir), execStart, restart, restartSec, systemdLogDirective(outLog), systemdLogDirective(errLog), strings.Join(environment, "\n"), m.savedTargetName())
}

func serviceMetadata(config AppConfig) string {
	args, _ := json.Marshal(configArguments(config))
	return strings.Join([]string{
		"# PM2Go-Script=" + encodeMetadataString(config.Script),
		"# PM2Go-Interpreter=" + encodeMetadataString(config.Interpreter),
		"# PM2Go-Args=" + base64.RawStdEncoding.EncodeToString(args),
		"# PM2Go-CreatedAt=" + strconv.FormatInt(config.CreatedAt, 10),
	}, "\n")
}

func configArguments(config AppConfig) AppArgs {
	if config.Argv != nil {
		return config.Argv
	}
	return splitCommandLine(config.Args)
}

func encodeMetadataString(value string) string {
	return base64.RawStdEncoding.EncodeToString([]byte(value))
}

func decodeMetadataString(value string) string {
	decoded, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil {
		return ""
	}
	return string(decoded)
}

// getServiceDir returns the directory where service files should be stored
func (m *Manager) getServiceDir() string {
	return m.serviceDir
}

func (m *Manager) withMutationLock(operation func() error) error {
	if m.initErr != nil {
		return m.initErr
	}
	serviceDir := m.getServiceDir()
	if err := os.MkdirAll(serviceDir, 0700); err != nil {
		return fmt.Errorf("failed to create service directory: %v", err)
	}

	lock, err := os.OpenFile(filepath.Join(serviceDir, ".pm2go.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open process registry lock: %v", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to lock process registry: %v", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	return operation()
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".pm2go-unit-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (m *Manager) prepareLogDirectories(config AppConfig) error {
	workingDir := config.Cwd
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}
	outLog, errLog := resolveLogPaths(config, workingDir)
	for _, path := range []string{outLog, errLog} {
		if path == "" || path == "/dev/null" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create log directory for %s: %v", path, err)
		}
	}
	return nil
}

// getCurrentUser returns the current username
func (m *Manager) getCurrentUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "nobody"
}

// systemdReload reloads the systemd daemon
func (m *Manager) systemdReload() error {
	return m.systemdCommand("daemon-reload")
}

// systemdCommand executes a systemctl command and waits for its result.
func (m *Manager) systemdCommand(args ...string) error {
	cmd := []string{"systemctl"}
	if m.userMode {
		cmd = append(cmd, "--user")
	}
	cmd = append(cmd, args...)

	return runCommand(cmd)
}

func runCommand(cmd []string) error {
	output, err := runCommandOutput(cmd)
	if err == nil {
		return nil
	}

	message := strings.TrimSpace(string(output))
	if message == "" {
		return err
	}
	return fmt.Errorf("%s: %w: %s", strings.Join(cmd, " "), err, message)
}

func runCommandOutput(cmd []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, cmd[0], cmd[1:]...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("command timed out after 30s")
	}
	return output, err
}

// getServicePID returns the PID of a service
func (m *Manager) getServicePID(serviceName string) int {
	cmd := []string{"systemctl"}
	if m.userMode {
		cmd = append(cmd, "--user")
	}
	cmd = append(cmd, "show", serviceName, "--property=MainPID", "--value")

	output, err := runCommandOutput(cmd)
	if err != nil {
		return 0
	}

	pidStr := strings.TrimSpace(string(output))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0
	}
	return pid
}

// getServiceStatus returns the status of a service
func (m *Manager) getServiceStatus(serviceName string) string {
	cmd := []string{"systemctl"}
	if m.userMode {
		cmd = append(cmd, "--user")
	}
	cmd = append(cmd, "show", serviceName, "--property=ActiveState", "--value")

	output, err := runCommandOutput(cmd)
	if err != nil {
		return "unknown"
	}

	status := strings.TrimSpace(string(output))
	return m.mapSystemdStatus(status)
}

// getServiceMemory returns memory usage in bytes for a given PID
func (m *Manager) getServiceMemory(pid int) int {
	if pid == 0 {
		return 0
	}

	// Read memory from /proc/PID/status
	statusFile := fmt.Sprintf("/proc/%d/status", pid)
	content, err := os.ReadFile(statusFile)
	if err != nil {
		return 0
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memKB, err := strconv.Atoi(fields[1])
				if err == nil {
					return memKB * 1024 // Convert KB to bytes
				}
			}
		}
	}
	return 0
}

// getServiceCPU returns CPU usage percentage for a given PID
func (m *Manager) getServiceCPU(pid int) int {
	if pid == 0 {
		return 0
	}

	// Read CPU stats from /proc/PID/stat
	statFile := fmt.Sprintf("/proc/%d/stat", pid)
	content, err := os.ReadFile(statFile)
	if err != nil {
		return 0
	}

	fields := strings.Fields(string(content))
	if len(fields) < 22 {
		return 0
	}

	// Get process CPU times (user + system time)
	utime, err1 := strconv.ParseInt(fields[13], 10, 64)     // user time
	stime, err2 := strconv.ParseInt(fields[14], 10, 64)     // system time
	starttime, err3 := strconv.ParseInt(fields[21], 10, 64) // process start time

	if err1 != nil || err2 != nil || err3 != nil {
		return 0
	}

	// Get system uptime to calculate process uptime
	uptimeContent, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}

	uptimeFields := strings.Fields(string(uptimeContent))
	if len(uptimeFields) < 1 {
		return 0
	}

	uptime, err := strconv.ParseFloat(uptimeFields[0], 64)
	if err != nil {
		return 0
	}

	// Calculate CPU usage
	// Clock ticks per second (usually 100)
	clockTicks := 100.0
	totalTime := float64(utime + stime)
	processUptime := uptime - (float64(starttime) / clockTicks)

	if processUptime <= 0 {
		return 0
	}

	cpuUsage := (totalTime / clockTicks / processUptime) * 100

	// Round and cap at 100%
	if cpuUsage > 100 {
		cpuUsage = 100
	}

	return int(cpuUsage + 0.5) // Round to nearest integer
}

// getServiceStartTime returns the active-enter time as epoch milliseconds.
func (m *Manager) getServiceStartTime(serviceName string) int64 {
	cmd := []string{"systemctl"}
	if m.userMode {
		cmd = append(cmd, "--user")
	}
	cmd = append(cmd, "show", serviceName, "--property=ActiveEnterTimestampMonotonic", "--value")

	output, err := runCommandOutput(cmd)
	if err != nil {
		return 0
	}

	monotonicMicros, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil || monotonicMicros <= 0 {
		return 0
	}

	uptimeData, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	uptimeFields := strings.Fields(string(uptimeData))
	if len(uptimeFields) == 0 {
		return 0
	}
	uptimeSeconds, err := strconv.ParseFloat(uptimeFields[0], 64)
	if err != nil {
		return 0
	}
	bootTime := time.Now().Add(-time.Duration(uptimeSeconds * float64(time.Second)))
	return bootTime.Add(time.Duration(monotonicMicros) * time.Microsecond).UnixMilli()
}

// mapSystemdStatus maps systemd status to PM2 status
func (m *Manager) mapSystemdStatus(status string) string {
	switch status {
	case "active":
		return "online"
	case "inactive":
		return "stopped"
	case "failed":
		return "errored"
	default:
		return "unknown"
	}
}

// Logs shows logs for a specific app or all apps
func (m *Manager) Logs(appName string, lines int, follow bool) error {
	if appName != "" {
		// Show logs for specific app from PM2-style log files
		return m.showAppLogs(appName, lines, follow)
	} else {
		// Show logs for all PM2go apps
		return m.showAllAppLogs(lines, follow)
	}
}

func (m *Manager) showAppLogs(appName string, lines int, follow bool) error {
	// Find the process to get log file paths
	processes, err := m.List()
	if err != nil {
		return fmt.Errorf("failed to get process list: %v", err)
	}

	var targetProcess *ProcessInfo
	for _, process := range processes {
		if process.Name == appName {
			targetProcess = &process
			break
		}
	}

	if targetProcess == nil {
		return fmt.Errorf("process '%s' not found", appName)
	}

	// Get log file paths
	outLogPath := targetProcess.PM2Env.PMOutLogPath
	errLogPath := targetProcess.PM2Env.PMErrLogPath

	if outLogPath == "" || errLogPath == "" {
		return fmt.Errorf("log paths not configured for process '%s'", appName)
	}

	// Use tail to show logs
	cmd := []string{"tail"}

	if lines > 0 {
		cmd = append(cmd, "-n", fmt.Sprintf("%d", lines))
	}

	if follow {
		cmd = append(cmd, "-f")
	}

	// Show both stdout and stderr
	cmd = append(cmd, outLogPath, errLogPath)

	// Execute tail command
	execCmd := exec.Command(cmd[0], cmd[1:]...)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	return execCmd.Run()
}

func (m *Manager) showAllAppLogs(lines int, follow bool) error {
	// Get all processes
	processes, err := m.List()
	if err != nil {
		return fmt.Errorf("failed to get process list: %v", err)
	}

	if len(processes) == 0 {
		fmt.Println("No processes found")
		return nil
	}

	// Collect all log file paths
	var logFiles []string
	for _, process := range processes {
		if process.PM2Env.PMOutLogPath != "" {
			logFiles = append(logFiles, process.PM2Env.PMOutLogPath)
		}
		if process.PM2Env.PMErrLogPath != "" {
			logFiles = append(logFiles, process.PM2Env.PMErrLogPath)
		}
	}

	if len(logFiles) == 0 {
		return fmt.Errorf("no log files found")
	}

	// Use tail to show logs from all files
	cmd := []string{"tail"}

	if lines > 0 {
		cmd = append(cmd, "-n", fmt.Sprintf("%d", lines))
	}

	if follow {
		cmd = append(cmd, "-f")
	}

	cmd = append(cmd, logFiles...)

	// Execute tail command
	execCmd := exec.Command(cmd[0], cmd[1:]...)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	return execCmd.Run()
}
