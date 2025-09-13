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
	return &Manager{
		userMode: os.Getuid() != 0, // use user services if not root
		prefix:   "pm2-",
	}
}

// serviceName returns the systemd service name for an app
func (m *Manager) serviceName(appName string) string {
	return m.prefix + appName
}

// serviceNameWithID returns the systemd service name with ID for an app
func (m *Manager) serviceNameWithID(id int, appName string) string {
	return fmt.Sprintf("%s%d-%s", m.prefix, id, appName)
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
		return 0, remaining, nil
	}
	
	idStr := remaining[:dashIndex]
	appName := remaining[dashIndex+1:]
	
	id, err := strconv.Atoi(idStr)
	if err != nil {
		// Old format without ID, treat whole thing as name
		return 0, remaining, nil
	}
	
	return id, appName, nil
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
	serviceName, err := m.findServiceByIdentifier(identifier)
	if err != nil {
		return err
	}
	return m.systemdCommand("stop", serviceName)
}

// Restart restarts an existing service by ID
func (m *Manager) Restart(id int) error {
	serviceName, err := m.findServiceByIdentifier(strconv.Itoa(id))
	if err != nil {
		return err
	}
	return m.systemdCommand("start", serviceName)
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

	var processes []ProcessInfo
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
			parts := strings.Fields(execStart)
			if len(parts) > 0 {
				config.Interpreter = parts[0]
				if len(parts) > 1 {
					config.Script = parts[1]
					if len(parts) > 2 {
						config.Args = strings.Join(parts[2:], " ")
					}
				}
			}
		} else if strings.HasPrefix(line, "Environment=") {
			envLine := strings.TrimPrefix(line, "Environment=")
			// Parse environment variables
			if strings.Contains(envLine, "=") {
				parts := strings.SplitN(envLine, "=", 2)
				if len(parts) == 2 {
					config.Env[parts[0]] = strings.Trim(parts[1], "\"")
				}
			}
		} else if strings.HasPrefix(line, "StandardOutput=") {
			outLine := strings.TrimPrefix(line, "StandardOutput=")
			if strings.HasPrefix(outLine, "append:") {
				config.OutLogPath = strings.TrimPrefix(outLine, "append:")
			}
		} else if strings.HasPrefix(line, "StandardError=") {
			errLine := strings.TrimPrefix(line, "StandardError=")
			if strings.HasPrefix(errLine, "append:") {
				config.ErrLogPath = strings.TrimPrefix(errLine, "append:")
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
		// Flush all logs
		cmd := []string{"journalctl"}
		if m.userMode {
			cmd = append(cmd, "--user")
		}
		cmd = append(cmd, "--rotate")
		return exec.Command(cmd[0], cmd[1:]...).Run()
	}

	// Flush specific app logs
	serviceName := m.serviceName(appName)
	cmd := []string{"journalctl"}
	if m.userMode {
		cmd = append(cmd, "--user")
	}
	cmd = append(cmd, "--unit", serviceName, "--rotate")
	return exec.Command(cmd[0], cmd[1:]...).Run()
}

// generateServiceFile creates the systemd service file content
func (m *Manager) generateServiceFile(config AppConfig) string {
	workingDir := config.Cwd
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}

	// Create PM2-style log directory
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".pm2", "logs")
	os.MkdirAll(logDir, 0755)
	
	outLog := filepath.Join(logDir, config.Name+"-out.log")
	errLog := filepath.Join(logDir, config.Name+"-error.log")

	var execStart string
	
	if config.Interpreter != "" {
		// Use explicit interpreter: "python3 script.py args"
		interpreterFields := strings.Fields(config.Interpreter)
		fullPath, err := exec.LookPath(interpreterFields[0])
		
		var interpreterPath string
		if err != nil {
			// Fallback to original interpreter if not found in PATH
			interpreterPath = interpreterFields[0]
		} else {
			interpreterPath = fullPath
		}
		
		// Add interpreter arguments if any
		if len(interpreterFields) > 1 {
			interpreterPath += " " + strings.Join(interpreterFields[1:], " ")
		}
		
		execStart = interpreterPath + " " + config.Script
		if config.Args != "" {
			execStart += " " + config.Args
		}
	} else {
		// Auto-detect interpreter or use script directly
		if strings.HasSuffix(config.Script, ".py") {
			pythonPath, err := exec.LookPath("python3")
			if err != nil {
				pythonPath = "python3"
			}
			execStart = pythonPath + " " + config.Script
		} else if strings.HasSuffix(config.Script, ".js") {
			nodePath, err := exec.LookPath("node")
			if err != nil {
				nodePath, err = exec.LookPath("nodejs")
				if err != nil {
					nodePath = "node"
				}
			}
			execStart = nodePath + " " + config.Script
		} else {
			// Use script directly (should have shebang)
			execStart = config.Script
		}
		
		if config.Args != "" {
			execStart += " " + config.Args
		}
	}

	var service string
	if m.userMode {
		// In user mode, don't specify User= as it causes "operation not permitted"
		service = fmt.Sprintf(`[Unit]
Description=PM2 App: %s
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=3
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`, config.Name, workingDir, execStart, outLog, errLog)
	} else {
		// In system mode, specify the user
		service = fmt.Sprintf(`[Unit]
Description=PM2 App: %s
After=network.target

[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=3
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`, config.Name, m.getCurrentUser(), workingDir, execStart, outLog, errLog)
	}

	// Add environment variables if present
	if config.Env != nil {
		envLines := ""
		for key, value := range config.Env {
			envLines += fmt.Sprintf("Environment=%s=%s\n", key, value)
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

	return exec.Command(cmd[0], cmd[1:]...).Run()
}

// systemdCommand executes a systemctl command
func (m *Manager) systemdCommand(action, serviceName string) error {
	cmd := []string{"systemctl"}
	if m.userMode {
		cmd = append(cmd, "--user")
	}
	cmd = append(cmd, action, serviceName)

	return exec.Command(cmd[0], cmd[1:]...).Run()
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
	utime, err1 := strconv.ParseInt(fields[13], 10, 64)  // user time
	stime, err2 := strconv.ParseInt(fields[14], 10, 64)  // system time
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