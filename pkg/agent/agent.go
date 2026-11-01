package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wojtekw92/pm2go/pkg/systemd"
)

type ProcessManager interface {
	List() ([]systemd.ProcessInfo, error)
	Start(systemd.AppConfig) error
	Stop(string) error
	Restart(int) error
	Delete(string) error
	Flush(string) error
	SendSignal(string, string) error
}

type Client struct {
	BaseURL    string
	Secret     string
	HTTPClient *http.Client
}

type HeartbeatRequest struct {
	MachineID string                `json:"machine_id"`
	Hostname  string                `json:"hostname"`
	Version   string                `json:"version"`
	Timestamp time.Time             `json:"timestamp"`
	Processes []systemd.ProcessInfo `json:"processes"`
	Results   []CommandResult       `json:"results,omitempty"`
}

type HeartbeatResponse struct {
	Commands []Command `json:"commands,omitempty"`
}

type Command struct {
	ID     string             `json:"id"`
	Action string             `json:"action"`
	Target string             `json:"target,omitempty"`
	Signal string             `json:"signal,omitempty"`
	Lines  int                `json:"lines,omitempty"`
	App    *systemd.AppConfig `json:"app,omitempty"`
}

type CommandResult struct {
	CommandID string `json:"command_id"`
	Action    string `json:"action"`
	Target    string `json:"target,omitempty"`
	OK        bool   `json:"ok"`
	Message   string `json:"message,omitempty"`
}

type Runner struct {
	Manager   ProcessManager
	Client    *Client
	MachineID string
	Hostname  string
	Version   string
	Results   []CommandResult
}

func (c *Client) Heartbeat(ctx context.Context, req HeartbeatRequest) (HeartbeatResponse, error) {
	var response HeartbeatResponse
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}

	endpoint, err := joinURL(c.BaseURL, "/api/v1/agents/heartbeat")
	if err != nil {
		return response, err
	}

	body, err := json.Marshal(req)
	if err != nil {
		return response, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return response, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.Secret != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.Secret)
	}

	httpResp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return response, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return response, fmt.Errorf("heartbeat failed with status %s", httpResp.Status)
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&response); err != nil {
		return response, err
	}

	return response, nil
}

func (r *Runner) Tick(ctx context.Context) error {
	processes, err := r.Manager.List()
	if err != nil {
		return err
	}

	response, err := r.Client.Heartbeat(ctx, HeartbeatRequest{
		MachineID: r.MachineID,
		Hostname:  r.Hostname,
		Version:   r.Version,
		Timestamp: time.Now().UTC(),
		Processes: sanitizeProcesses(processes),
		Results:   r.Results,
	})
	if err != nil {
		return err
	}

	r.Results = r.Results[:0]
	for _, command := range response.Commands {
		r.Results = append(r.Results, ExecuteCommand(r.Manager, command))
	}

	return nil
}

func sanitizeProcesses(processes []systemd.ProcessInfo) []systemd.ProcessInfo {
	sanitized := make([]systemd.ProcessInfo, len(processes))
	copy(sanitized, processes)
	for i := range sanitized {
		sanitized[i].PM2Env.Env = nil
	}
	return sanitized
}

func ExecuteCommand(manager ProcessManager, command Command) CommandResult {
	result := CommandResult{
		CommandID: command.ID,
		Action:    command.Action,
		Target:    command.Target,
		OK:        true,
	}

	var err error
	switch strings.ToLower(command.Action) {
	case "start":
		if command.App == nil {
			err = fmt.Errorf("start command requires app config")
		} else {
			err = manager.Start(*command.App)
		}
	case "stop":
		err = manager.Stop(command.Target)
	case "restart", "reload":
		err = restartTarget(manager, command.Target)
	case "delete":
		err = manager.Delete(command.Target)
	case "flush":
		err = manager.Flush(command.Target)
	case "signal", "sendsignal":
		err = manager.SendSignal(command.Target, NormalizeSignal(command.Signal))
	case "inspect", "describe":
		result.Message, err = inspectTarget(manager, command.Target)
	case "logs":
		result.Message, err = readTargetLogs(manager, command.Target, command.Lines)
	default:
		err = fmt.Errorf("unsupported command action %q", command.Action)
	}

	if err != nil {
		result.OK = false
		result.Message = err.Error()
	} else if result.Message == "" {
		result.Message = "ok"
	}

	return result
}

func inspectTarget(manager ProcessManager, target string) (string, error) {
	process, err := findProcess(manager, target)
	if err != nil {
		return "", err
	}
	process.PM2Env.Env = nil
	data, err := json.MarshalIndent(process, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func readTargetLogs(manager ProcessManager, target string, lines int) (string, error) {
	process, err := findProcess(manager, target)
	if err != nil {
		return "", err
	}
	if lines <= 0 {
		lines = 100
	}
	if lines > 500 {
		lines = 500
	}

	sections := []string{}
	paths := []struct {
		label string
		path  string
	}{
		{label: "stdout", path: process.PM2Env.PMOutLogPath},
		{label: "stderr", path: process.PM2Env.PMErrLogPath},
	}

	for _, logFile := range paths {
		if logFile.path == "" {
			continue
		}
		content, err := os.ReadFile(logFile.path)
		if err != nil {
			sections = append(sections, fmt.Sprintf("==> %s (%s) <==\n%s", logFile.label, logFile.path, err.Error()))
			continue
		}
		sections = append(sections, fmt.Sprintf("==> %s (%s) <==\n%s", logFile.label, logFile.path, lastLines(string(content), lines)))
	}

	if len(sections) == 0 {
		return "", fmt.Errorf("process %q has no log paths", target)
	}
	return strings.Join(sections, "\n\n"), nil
}

func findProcess(manager ProcessManager, target string) (systemd.ProcessInfo, error) {
	processes, err := manager.List()
	if err != nil {
		return systemd.ProcessInfo{}, err
	}
	for _, process := range processes {
		if process.Name == target || strconv.Itoa(process.PM2Env.ID) == target {
			return process, nil
		}
	}
	return systemd.ProcessInfo{}, fmt.Errorf("process %q not found", target)
}

func lastLines(content string, limit int) string {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= limit {
		return content
	}
	return strings.Join(lines[len(lines)-limit:], "\n")
}

func NormalizeSignal(signal string) string {
	signal = strings.TrimSpace(strings.ToUpper(signal))
	if signal == "" || strings.HasPrefix(signal, "SIG") {
		return signal
	}
	return "SIG" + signal
}

func restartTarget(manager ProcessManager, target string) error {
	processes, err := manager.List()
	if err != nil {
		return err
	}

	if target == "all" {
		var errors []string
		for _, process := range processes {
			if err := manager.Restart(process.PM2Env.ID); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", process.Name, err))
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("failed to restart processes: %s", strings.Join(errors, "; "))
		}
		return nil
	}

	for _, process := range processes {
		if process.Name == target || strconv.Itoa(process.PM2Env.ID) == target {
			return manager.Restart(process.PM2Env.ID)
		}
	}

	return fmt.Errorf("process %q not found", target)
}

func joinURL(baseURL string, path string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("api url must include scheme and host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	return parsed.String(), nil
}
