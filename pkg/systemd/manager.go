package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Manager handles systemd operations for PM2-style process management
type Manager struct {
	userMode bool
	prefix   string // prefix for service names to avoid conflicts
}

// NewManager creates a new systemd manager instance
func NewManager() *Manager {
	prefix := os.Getenv("PM2GO_SERVICE_PREFIX")
	if prefix == "" {
		prefix = "pm2-"
	}

	return &Manager{
		userMode: os.Getuid() != 0, // use user services if not root
		prefix:   prefix,
	}
}

// serviceName returns the systemd service name for an app
func (m *Manager) serviceName(appName string) string {
	return m.prefix + escapeUnitNamePart(appName)
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

// getNextAvailableID finds the next available ID
func (m *Manager) getNextAvailableID() int {
	processes, err := m.List()
	if err != nil {
		return 0
	}

	maxID := -1
	for _, process := range processes {
		if process.PM2Env.ID > maxID {
			maxID = process.PM2Env.ID
		}
	}

	return maxID + 1
}

// findServiceByIdentifier finds service name by ID or name
func (m *Manager) findServiceByIdentifier(identifier string) (string, error) {
	processes, err := m.List()
	if err != nil {
		return "", err
	}

	// Try to parse as ID first
	if id, err := strconv.Atoi(identifier); err == nil {
		for _, process := range processes {
			if process.PM2Env.ID == id {
				return m.serviceNameWithID(process.PM2Env.ID, process.Name), nil
			}
		}
		return "", fmt.Errorf("process with ID %d not found", id)
	}

	// Try as name
	for _, process := range processes {
		if process.Name == identifier {
			return m.serviceNameWithID(process.PM2Env.ID, process.Name), nil
		}
	}

	return "", fmt.Errorf("process '%s' not found", identifier)
}

// checkDuplicateName checks if a name already exists among running processes
func (m *Manager) checkDuplicateName(name string) error {
	processes, err := m.List()
	if err != nil {
		return err
	}

	for _, process := range processes {
		if process.Name == name && process.PM2Env.Status != "stopped" {
			return fmt.Errorf("process with name '%s' already exists (ID: %d)", name, process.PM2Env.ID)
		}
	}

	return nil
}

// Start creates and starts a systemd service for the given app config
func (m *Manager) Start(config AppConfig) error {
	// Check for duplicate names
	if err := m.checkDuplicateName(config.Name); err != nil {
		return err
	}

	// Assign ID if not set
	if config.ID == 0 {
		config.ID = m.getNextAvailableID()
	}

	serviceName := m.serviceNameWithID(config.ID, config.Name)

	// Generate systemd service file content
	serviceContent := m.generateServiceFile(config)

	// Write service file
	serviceDir := m.getServiceDir()
	servicePath := filepath.Join(serviceDir, serviceName+".service")

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %v", err)
	}

	// Reload systemd and start service
	if err := m.systemdReload(); err != nil {
		return fmt.Errorf("failed to reload systemd: %v", err)
	}

	if err := m.systemdCommand("start", serviceName); err != nil {
		return fmt.Errorf("failed to start service: %v", err)
	}

	if err := m.systemdCommand("enable", serviceName); err != nil {
		return fmt.Errorf("failed to enable service: %v", err)
	}

	return nil
}

// Stop stops a systemd service
func (m *Manager) Stop(identifier string) error {
	if identifier == "all" {
		return m.stopAll()
	}

	serviceName, err := m.findServiceByIdentifier(identifier)
	if err != nil {
		return err
	}
	return m.systemdCommand("stop", serviceName)
}

// stopAll stops all pm2go managed services without deleting their unit files.
func (m *Manager) stopAll() error {
	processes, err := m.List()
	if err != nil {
		return err
	}

	var errors []string
	for _, process := range processes {
		serviceName := m.serviceNameWithID(process.PM2Env.ID, process.Name)
		if err := m.systemdCommand("stop", serviceName); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", process.Name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to stop services: %s", strings.Join(errors, "; "))
	}
	return nil
}

// Restart restarts an existing service by ID
func (m *Manager) Restart(id int) error {
	serviceName, err := m.findServiceByIdentifier(strconv.Itoa(id))
	if err != nil {
		return err
	}
	return m.systemdCommand("restart", serviceName)
}

// SendSignal sends a Unix signal to a managed service's main process.
func (m *Manager) SendSignal(identifier string, signal string) error {
	serviceName, err := m.findServiceByIdentifier(identifier)
	if err != nil {
		return err
	}

	cmd := []string{"systemctl"}
	if m.userMode {
		cmd = append(cmd, "--user")
	}
	cmd = append(cmd, "kill", "--kill-who=main", "--signal="+signal, serviceName)

	return runCommand(cmd)
}

// Delete stops and removes a systemd service
func (m *Manager) Delete(identifier string) error {
	if identifier == "all" {
		return m.deleteAll()
	}

	// Find service by name or ID
	serviceName, err := m.findServiceByIdentifier(identifier)
	if err != nil {
		return err
	}

	// Stop service first
	m.systemdCommand("stop", serviceName)
	m.systemdCommand("disable", serviceName)

	// Remove service file
	serviceDir := m.getServiceDir()
	servicePath := filepath.Join(serviceDir, serviceName+".service")
	os.Remove(servicePath)

	return m.systemdReload()
}

// deleteAll removes all pm2go managed services
func (m *Manager) deleteAll() error {
	processes, err := m.List()
	if err != nil {
		return err
	}

	for _, process := range processes {
		serviceName := m.serviceNameWithID(process.PM2Env.ID, process.Name)
		m.systemdCommand("stop", serviceName)
		m.systemdCommand("disable", serviceName)

		serviceDir := m.getServiceDir()
		servicePath := filepath.Join(serviceDir, serviceName+".service")
		os.Remove(servicePath)
	}

	return m.systemdReload()
}

// List returns a list of all managed processes
func (m *Manager) List() ([]ProcessInfo, error) {
	// Get all service files instead of just active units
	serviceDir := m.getServiceDir()
	files, err := filepath.Glob(filepath.Join(serviceDir, m.prefix+"*.service"))
	if err != nil {
		return nil, err
	}

	processes := []ProcessInfo{}
	for _, filePath := range files {
		fileName := filepath.Base(filePath)
		serviceName := strings.TrimSuffix(fileName, ".service")

		// Parse ID and name from service name
		id, appName, err := m.parseServiceName(serviceName)
		if err != nil {
			continue // Skip invalid service names
		}

		// Get service status
		status := m.getServiceStatus(serviceName)

		// Get PID for the service (will be 0 if stopped)
		pid := m.getServicePID(serviceName)

		// Get memory usage, CPU, and uptime (will be 0 if stopped)
		memory := m.getServiceMemory(pid)
		cpu := m.getServiceCPU(pid)
		uptime := m.getServiceUptime(serviceName)
		createdAt := time.Now().Unix()*1000 - uptime

		// Read service configuration for additional details
		config := m.readServiceConfig(serviceName)

		process := ProcessInfo{
			PID:  pid,
			Name: appName,
			PM2Env: PM2Env{
				ID:               id,
				Name:             appName,
				ExecMode:         "fork",
				Status:           status,
				PMUptime:         uptime,
				CreatedAt:        createdAt,
				RestartTime:      0,
				UnstableRestarts: 0,
				Versioning:       nil,
				Node: PM2Node{
					Version: "unknown",
				},
				PMExecPath:   config.Script,
				PMCwd:        config.Cwd,
				PMOutLogPath: config.OutLogPath,
				PMErrLogPath: config.ErrLogPath,
				PMPidPath:    config.PidPath,
				Interpreter:  config.Interpreter,
				Args:         config.Args,
				Env:          config.Env,
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
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "ExecStart=") {
			execStart := strings.TrimPrefix(line, "ExecStart=")
			// Parse the exec command to extract script and interpreter
			parts := splitCommandLine(execStart)
			if len(parts) > 0 {
				config.Interpreter = parts[0]
				if len(parts) > 1 {
					config.Script = parts[1]
					if len(parts) > 2 {
						config.Args = strings.Join(parts[2:], " ")
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

	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
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
			continue
		}

		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(r)
	}

	if escaped {
		current.WriteRune('\\')
	}
	if current.Len() > 0 {
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
		return "3"
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
		if config.Args != "" {
			execArgs = append(execArgs, splitCommandLine(config.Args)...)
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

		if config.Args != "" {
			execArgs = append(execArgs, splitCommandLine(config.Args)...)
		}
	}

	execStart := joinSystemdArgs(execArgs)

	var service string
	if m.userMode {
		// In user mode, don't specify User= as it causes "operation not permitted"
		service = fmt.Sprintf(`[Unit]
Description=PM2 App: %s
After=network.target
%s

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s
Restart=%s
RestartSec=%s
StandardOutput=%s
StandardError=%s

[Install]
WantedBy=default.target
`, config.Name, startLimits, systemdPathValue(workingDir), execStart, restart, restartSec, systemdLogDirective(outLog), systemdLogDirective(errLog))
	} else {
		// In system mode, specify the user
		service = fmt.Sprintf(`[Unit]
Description=PM2 App: %s
After=network.target
%s

[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=%s
Restart=%s
RestartSec=%s
StandardOutput=%s
StandardError=%s

[Install]
WantedBy=default.target
`, config.Name, startLimits, m.getCurrentUser(), systemdPathValue(workingDir), execStart, restart, restartSec, systemdLogDirective(outLog), systemdLogDirective(errLog))
	}

	// Add environment variables if present
	if config.Env != nil {
		envLines := ""
		for key, value := range config.Env {
			envLines += fmt.Sprintf("Environment=%s\n", quoteSystemdValue(key+"="+value))
		}
		service = strings.Replace(service, "[Install]", envLines+"\n[Install]", 1)
	}

	return service
}

// getServiceDir returns the directory where service files should be stored
func (m *Manager) getServiceDir() string {
	if m.userMode {
		home, _ := os.UserHomeDir()
		dir := filepath.Join(home, ".config/systemd/user")
		os.MkdirAll(dir, 0755)
		return dir
	}
	return "/etc/systemd/system"
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
	cmd := []string{"systemctl"}
	if m.userMode {
		cmd = append(cmd, "--user")
	}
	cmd = append(cmd, "daemon-reload")

	return runCommand(cmd)
}

// systemdCommand executes a systemctl command
func (m *Manager) systemdCommand(action, serviceName string) error {
	cmd := []string{"systemctl"}
	if m.userMode {
		cmd = append(cmd, "--user")
	}
	cmd = append(cmd, action, serviceName)

	return runCommand(cmd)
}

func runCommand(cmd []string) error {
	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err == nil {
		return nil
	}

	message := strings.TrimSpace(string(output))
	if message == "" {
		return err
	}
	return fmt.Errorf("%s: %w: %s", strings.Join(cmd, " "), err, message)
}

// getServicePID returns the PID of a service
func (m *Manager) getServicePID(serviceName string) int {
	cmd := []string{"systemctl"}
	if m.userMode {
		cmd = append(cmd, "--user")
	}
	cmd = append(cmd, "show", serviceName, "--property=MainPID", "--value")

	output, err := exec.Command(cmd[0], cmd[1:]...).Output()
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
	cmd = append(cmd, "is-active", serviceName)

	output, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		// Check if service is loaded but not active
		return "stopped"
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

// getServiceUptime returns uptime in milliseconds for a service
func (m *Manager) getServiceUptime(serviceName string) int64 {
	cmd := []string{"systemctl"}
	if m.userMode {
		cmd = append(cmd, "--user")
	}
	cmd = append(cmd, "show", serviceName, "--property=ActiveEnterTimestamp", "--value")

	output, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return 0
	}

	timestamp := strings.TrimSpace(string(output))
	if timestamp == "" || timestamp == "n/a" {
		return 0
	}

	// Parse systemd timestamp format
	startTime, err := time.Parse("Mon 2006-01-02 15:04:05 MST", timestamp)
	if err != nil {
		return 0
	}

	uptime := time.Since(startTime)
	return int64(uptime.Milliseconds())
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
