package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/wojtekw92/pm2go/pkg/agent"
	"github.com/wojtekw92/pm2go/pkg/systemd"
)

const agentVersion = "0.1"

var agentAPIURL string
var agentSecret string
var agentMachineID string
var agentInterval time.Duration
var agentDaemon bool
var agentPIDFile string
var agentLogFile string

const agentServiceName = "pm2go-agent.service"

var agentCmd = &cobra.Command{
	Use:   "agent [start|once|startup|stop|status]",
	Short: "Run outbound PM2go cloud agent",
	Long: `Run an outbound PM2go agent on a process host.

The agent does not open inbound ports. It posts process inventory and command
results to a central API endpoint and executes commands returned by that API.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		action := "start"
		if len(args) > 0 {
			action = args[0]
		}

		switch action {
		case "start":
			handleAgentStart()
		case "once":
			handleAgentOnce()
		case "startup":
			handleAgentStartup()
		case "stop":
			handleAgentStop()
		case "status":
			handleAgentStatus()
		default:
			fmt.Printf("Unknown agent action: %s\n", action)
			os.Exit(1)
		}
	},
}

func handleAgentStartup() {
	applyAgentDefaults()
	requireAgentConfig()

	if err := os.MkdirAll(filepath.Dir(agentLogFile), 0755); err != nil {
		fmt.Printf("Error creating log directory: %v\n", err)
		os.Exit(1)
	}

	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("Error resolving executable: %v\n", err)
		os.Exit(1)
	}

	serviceDir := userSystemdDir()
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		fmt.Printf("Error creating systemd user directory: %v\n", err)
		os.Exit(1)
	}

	servicePath := filepath.Join(serviceDir, agentServiceName)
	envFilePath := defaultAgentEnvFilePath()
	if err := os.MkdirAll(filepath.Dir(envFilePath), 0700); err != nil {
		fmt.Printf("Error creating environment directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(envFilePath, []byte(generateAgentEnvironmentFile(agentAPIURL, agentSecret, agentMachineID)), 0600); err != nil {
		fmt.Printf("Error writing %s: %v\n", envFilePath, err)
		os.Exit(1)
	}

	service := generateAgentServiceFile(agentServiceConfig{
		Executable: executable,
		EnvFile:    envFilePath,
		Interval:   agentInterval,
		LogFile:    agentLogFile,
	})

	if err := os.WriteFile(servicePath, []byte(service), 0644); err != nil {
		fmt.Printf("Error writing %s: %v\n", servicePath, err)
		os.Exit(1)
	}

	if err := runSystemctlUser("daemon-reload"); err != nil {
		fmt.Printf("Error reloading systemd user daemon: %v\n", err)
		os.Exit(1)
	}
	if err := runSystemctlUser("enable", "--now", agentServiceName); err != nil {
		fmt.Printf("Error enabling agent service: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Installed and started %s\n", agentServiceName)
	fmt.Printf("  Service file: %s\n", servicePath)
	fmt.Printf("  Environment file: %s\n", envFilePath)
	fmt.Printf("  Central API: %s\n", agentAPIURL)
	fmt.Printf("  Machine ID: %s\n", agentMachineID)
	fmt.Printf("  Log file: %s\n", agentLogFile)
	fmt.Println()
	fmt.Println("If this user service must survive logout/reboot, ensure PM2go startup is configured:")
	fmt.Println("  pm2go startup")
}

func init() {
	agentCmd.Flags().StringVar(&agentAPIURL, "api-url", "", "Central API base URL (or PM2GO_API_URL)")
	agentCmd.Flags().StringVar(&agentSecret, "secret", "", "Bearer token for central API (or PM2GO_AGENT_SECRET)")
	agentCmd.Flags().StringVar(&agentMachineID, "machine-id", "", "Stable machine ID reported to central API")
	agentCmd.Flags().DurationVar(&agentInterval, "interval", 15*time.Second, "Heartbeat interval")
	agentCmd.Flags().BoolVar(&agentDaemon, "daemon", false, "Run agent in background")
	agentCmd.Flags().StringVar(&agentPIDFile, "pid-file", "", "Daemon PID file path")
	agentCmd.Flags().StringVar(&agentLogFile, "log-file", "", "Daemon log file path")
}

func handleAgentStart() {
	applyAgentDefaults()
	requireAgentConfig()

	if agentDaemon {
		startAgentDaemon()
		return
	}

	runAgentLoop()
}

func handleAgentOnce() {
	applyAgentDefaults()
	requireAgentConfig()
	runner := newAgentRunner()
	if err := runner.Tick(context.Background()); err != nil {
		fmt.Printf("Agent heartbeat failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Agent heartbeat completed")
}

func runAgentLoop() {
	runner := newAgentRunner()
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	ticker := time.NewTicker(agentInterval)
	defer ticker.Stop()

	fmt.Printf("PM2go agent started: api=%s machine_id=%s interval=%s\n", agentAPIURL, agentMachineID, agentInterval)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), agentInterval)
		if err := runner.Tick(ctx); err != nil {
			fmt.Printf("Agent heartbeat failed: %v\n", err)
		}
		cancel()

		select {
		case <-done:
			fmt.Println("PM2go agent stopped")
			return
		case <-ticker.C:
		}
	}
}

func newAgentRunner() *agent.Runner {
	hostname, _ := os.Hostname()
	return &agent.Runner{
		Manager:   systemd.NewManager(),
		Client:    &agent.Client{BaseURL: agentAPIURL, Secret: agentSecret, HTTPClient: &http.Client{Timeout: 15 * time.Second}},
		MachineID: agentMachineID,
		Hostname:  hostname,
		Version:   agentVersion,
	}
}

func startAgentDaemon() {
	if pid, running := readRunningAgentPID(agentPIDFile); running {
		fmt.Printf("PM2go agent is already running with PID %d\n", pid)
		return
	}

	if err := os.MkdirAll(filepath.Dir(agentPIDFile), 0755); err != nil {
		fmt.Printf("Error creating PID directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(agentLogFile), 0755); err != nil {
		fmt.Printf("Error creating log directory: %v\n", err)
		os.Exit(1)
	}

	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("Error resolving executable: %v\n", err)
		os.Exit(1)
	}

	logFile, err := os.OpenFile(agentLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		fmt.Printf("Error opening %s: %v\n", os.DevNull, err)
		os.Exit(1)
	}
	defer devNull.Close()

	args := []string{
		executable, "agent", "start",
		"--api-url", agentAPIURL,
		"--machine-id", agentMachineID,
		"--interval", agentInterval.String(),
		"--pid-file", agentPIDFile,
		"--log-file", agentLogFile,
	}
	env := os.Environ()
	if agentSecret != "" {
		env = append(env, "PM2GO_AGENT_SECRET="+agentSecret)
	}

	process, err := os.StartProcess(executable, args, &os.ProcAttr{
		Files: []*os.File{devNull, logFile, logFile},
		Env:   env,
		Sys:   &syscall.SysProcAttr{Setsid: true},
	})
	if err != nil {
		fmt.Printf("Error starting agent daemon: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(agentPIDFile, []byte(strconv.Itoa(process.Pid)+"\n"), 0644); err != nil {
		fmt.Printf("Error writing PID file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("PM2go agent started with PID %d\n", process.Pid)
	fmt.Printf("Central API: %s\n", agentAPIURL)
	fmt.Printf("Log file: %s\n", agentLogFile)
}

func handleAgentStop() {
	applyAgentDefaults()
	if agentServiceInstalled() {
		if err := runSystemctlUser("stop", agentServiceName); err != nil {
			fmt.Printf("Error stopping %s: %v\n", agentServiceName, err)
			os.Exit(1)
		}
		fmt.Printf("PM2go agent service stopped (%s)\n", agentServiceName)
		return
	}

	pid, running := readRunningAgentPID(agentPIDFile)
	if !running {
		fmt.Println("PM2go agent is not running")
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("Error finding agent process: %v\n", err)
		os.Exit(1)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Printf("Error stopping agent: %v\n", err)
		os.Exit(1)
	}
	_ = os.Remove(agentPIDFile)
	fmt.Printf("PM2go agent stopped (PID %d)\n", pid)
}

func handleAgentStatus() {
	applyAgentDefaults()
	if agentServiceInstalled() {
		status := "inactive"
		if output, err := exec.Command("systemctl", "--user", "is-active", agentServiceName).Output(); err == nil {
			status = strings.TrimSpace(string(output))
		}
		fmt.Printf("PM2go agent service is %s (%s)\n", status, agentServiceName)
		if status != "active" {
			os.Exit(1)
		}
		return
	}

	pid, running := readRunningAgentPID(agentPIDFile)
	if !running {
		fmt.Println("PM2go agent is stopped")
		os.Exit(1)
	}
	fmt.Printf("PM2go agent is running with PID %d\n", pid)
}

type agentServiceConfig struct {
	Executable string
	EnvFile    string
	Interval   time.Duration
	LogFile    string
}

func generateAgentServiceFile(config agentServiceConfig) string {
	execStart := strings.Join([]string{
		quoteSystemdArg(config.Executable),
		"agent",
		"start",
		"--interval",
		quoteSystemdArg(config.Interval.String()),
	}, " ")

	return fmt.Sprintf(`[Unit]
Description=PM2go outbound agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=%s
ExecStart=%s
Restart=always
RestartSec=5
StandardOutput=%s
StandardError=%s

[Install]
WantedBy=default.target
`, systemdUnitPathValue(config.EnvFile), execStart, "append:"+systemdUnitPathValue(config.LogFile), "append:"+systemdUnitPathValue(config.LogFile))
}

func generateAgentEnvironmentFile(apiURL string, secret string, machineID string) string {
	return fmt.Sprintf("PM2GO_API_URL=%s\nPM2GO_AGENT_SECRET=%s\nPM2GO_MACHINE_ID=%s\n",
		quoteEnvironmentFileValue(apiURL),
		quoteEnvironmentFileValue(secret),
		quoteEnvironmentFileValue(machineID),
	)
}

func defaultAgentEnvFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pm2go", "agent.env")
}

func userSystemdDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user")
}

func agentServiceInstalled() bool {
	_, err := os.Stat(filepath.Join(userSystemdDir(), agentServiceName))
	return err == nil
}

func runSystemctlUser(args ...string) error {
	cmdArgs := append([]string{"--user"}, args...)
	output, err := exec.Command("systemctl", cmdArgs...).CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(output))
	if message == "" {
		return err
	}
	return fmt.Errorf("systemctl %s: %w: %s", strings.Join(cmdArgs, " "), err, message)
}

func quoteSystemdArg(value string) string {
	escaped := escapeSystemdSpecifiers(value)
	escaped = strings.ReplaceAll(escaped, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	escaped = strings.ReplaceAll(escaped, "\n", "\\n")
	return "\"" + escaped + "\""
}

func escapeSystemdSpecifiers(value string) string {
	return strings.ReplaceAll(value, "%", "%%")
}

func quoteEnvironmentFileValue(value string) string {
	escaped := strings.ReplaceAll(value, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	escaped = strings.ReplaceAll(escaped, "\n", "\\n")
	return "\"" + escaped + "\""
}

func systemdUnitPathValue(value string) string {
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

func applyAgentDefaults() {
	if agentAPIURL == "" {
		agentAPIURL = os.Getenv("PM2GO_API_URL")
	}
	if agentSecret == "" {
		agentSecret = os.Getenv("PM2GO_AGENT_SECRET")
	}
	if agentMachineID == "" {
		agentMachineID = defaultAgentMachineID()
	}
	if agentPIDFile == "" {
		home, _ := os.UserHomeDir()
		agentPIDFile = filepath.Join(home, ".pm2", "pm2go-agent.pid")
	}
	if agentLogFile == "" {
		home, _ := os.UserHomeDir()
		agentLogFile = filepath.Join(home, ".pm2", "pm2go-agent.log")
	}
}

func requireAgentConfig() {
	if agentAPIURL == "" {
		fmt.Println("Error: --api-url or PM2GO_API_URL is required")
		os.Exit(1)
	}
	if err := validateAgentAPIURL(agentAPIURL); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if agentSecret == "" {
		fmt.Println("Warning: agent secret is empty; set --secret or PM2GO_AGENT_SECRET for production")
	}
}

func validateAgentAPIURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return fmt.Errorf("http API URLs are only allowed for loopback hosts; use https for %s", parsed.Hostname())
	}
	return nil
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func defaultAgentMachineID() string {
	if machineID := os.Getenv("PM2GO_MACHINE_ID"); machineID != "" {
		return machineID
	}
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		return hostname
	}
	return "unknown"
}

func readRunningAgentPID(pidFile string) (int, bool) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, false
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return 0, false
	}
	return pid, true
}
