package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wojtekw92/pm2go/pkg/systemd"
)

type fakeManager struct {
	processes    []systemd.ProcessInfo
	started      systemd.AppConfig
	stopped      string
	restarted    int
	deleted      string
	flushed      string
	signalTarget string
	signalName   string
}

func (f *fakeManager) List() ([]systemd.ProcessInfo, error) {
	return f.processes, nil
}

func (f *fakeManager) Start(config systemd.AppConfig) error {
	f.started = config
	return nil
}

func (f *fakeManager) Stop(identifier string) error {
	f.stopped = identifier
	return nil
}

func (f *fakeManager) Restart(id int) error {
	f.restarted = id
	return nil
}

func (f *fakeManager) Delete(identifier string) error {
	f.deleted = identifier
	return nil
}

func (f *fakeManager) Flush(identifier string) error {
	f.flushed = identifier
	return nil
}

func (f *fakeManager) SendSignal(identifier string, signal string) error {
	f.signalTarget = identifier
	f.signalName = signal
	return nil
}

func TestHeartbeatPostsInventoryAndReadsCommands(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/heartbeat" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected auth header %q", r.Header.Get("Authorization"))
		}

		var heartbeat HeartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&heartbeat); err != nil {
			t.Fatalf("failed to decode heartbeat: %v", err)
		}
		if heartbeat.MachineID != "machine-1" || len(heartbeat.Processes) != 1 {
			t.Fatalf("unexpected heartbeat: %#v", heartbeat)
		}

		_ = json.NewEncoder(w).Encode(HeartbeatResponse{Commands: []Command{{ID: "cmd-1", Action: "stop", Target: "api"}}})
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, Secret: "secret", HTTPClient: server.Client()}
	response, err := client.Heartbeat(context.Background(), HeartbeatRequest{
		MachineID: "machine-1",
		Processes: []systemd.ProcessInfo{{Name: "api"}},
	})
	if err != nil {
		t.Fatalf("expected heartbeat to succeed: %v", err)
	}
	if len(response.Commands) != 1 || response.Commands[0].Action != "stop" {
		t.Fatalf("unexpected commands: %#v", response.Commands)
	}
}

func TestRunnerExecutesReturnedCommandsAndReportsOnNextTick(t *testing.T) {
	heartbeatCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		heartbeatCount++
		var heartbeat HeartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&heartbeat); err != nil {
			t.Fatalf("failed to decode heartbeat: %v", err)
		}

		if heartbeatCount == 1 {
			_ = json.NewEncoder(w).Encode(HeartbeatResponse{Commands: []Command{{ID: "cmd-1", Action: "restart", Target: "api"}}})
			return
		}

		if len(heartbeat.Results) != 1 || !heartbeat.Results[0].OK || heartbeat.Results[0].CommandID != "cmd-1" {
			t.Fatalf("expected command result on second heartbeat, got %#v", heartbeat.Results)
		}
		_ = json.NewEncoder(w).Encode(HeartbeatResponse{})
	}))
	defer server.Close()

	manager := &fakeManager{processes: []systemd.ProcessInfo{{Name: "api", PM2Env: systemd.PM2Env{ID: 4, Name: "api"}}}}
	runner := &Runner{
		Manager:   manager,
		Client:    &Client{BaseURL: server.URL, HTTPClient: server.Client()},
		MachineID: "machine-1",
	}

	if err := runner.Tick(context.Background()); err != nil {
		t.Fatalf("first tick failed: %v", err)
	}
	if manager.restarted != 4 {
		t.Fatalf("expected restart ID 4, got %d", manager.restarted)
	}
	if err := runner.Tick(context.Background()); err != nil {
		t.Fatalf("second tick failed: %v", err)
	}
}

func TestRunnerRedactsProcessEnvFromHeartbeat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var heartbeat HeartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&heartbeat); err != nil {
			t.Fatalf("failed to decode heartbeat: %v", err)
		}
		if len(heartbeat.Processes) != 1 || heartbeat.Processes[0].PM2Env.Env != nil {
			t.Fatalf("expected process env to be redacted, got %#v", heartbeat.Processes)
		}
		_ = json.NewEncoder(w).Encode(HeartbeatResponse{})
	}))
	defer server.Close()

	runner := &Runner{
		Manager: &fakeManager{processes: []systemd.ProcessInfo{{Name: "api", PM2Env: systemd.PM2Env{Env: map[string]string{"SECRET": "value"}}}}},
		Client:  &Client{BaseURL: server.URL, HTTPClient: server.Client()},
	}
	if err := runner.Tick(context.Background()); err != nil {
		t.Fatalf("tick failed: %v", err)
	}
}

func TestExecuteCommandStart(t *testing.T) {
	manager := &fakeManager{}
	result := ExecuteCommand(manager, Command{ID: "cmd-1", Action: "start", App: &systemd.AppConfig{Name: "api", Script: "app.js"}})
	if !result.OK {
		t.Fatalf("expected command to succeed: %#v", result)
	}
	if manager.started.Name != "api" || manager.started.Script != "app.js" {
		t.Fatalf("unexpected started config: %#v", manager.started)
	}
}

func TestExecuteCommandSignalNormalizesSignal(t *testing.T) {
	manager := &fakeManager{}
	result := ExecuteCommand(manager, Command{ID: "cmd-1", Action: "signal", Target: "api", Signal: "usr2"})
	if !result.OK {
		t.Fatalf("expected command to succeed: %#v", result)
	}
	if manager.signalTarget != "api" || manager.signalName != "SIGUSR2" {
		t.Fatalf("unexpected signal call: target=%q signal=%q", manager.signalTarget, manager.signalName)
	}
}

func TestExecuteCommandInspectReturnsProcessJSON(t *testing.T) {
	manager := &fakeManager{processes: []systemd.ProcessInfo{{Name: "api", PID: 123, PM2Env: systemd.PM2Env{ID: 7, Name: "api"}}}}
	result := ExecuteCommand(manager, Command{ID: "cmd-1", Action: "inspect", Target: "api"})
	if !result.OK {
		t.Fatalf("expected command to succeed: %#v", result)
	}
	if !strings.Contains(result.Message, `"name": "api"`) || !strings.Contains(result.Message, `"pid": 123`) {
		t.Fatalf("unexpected inspect output: %s", result.Message)
	}
}

func TestExecuteCommandLogsReturnsRecentLogLines(t *testing.T) {
	dir := t.TempDir()
	outLog := filepath.Join(dir, "api-out.log")
	errLog := filepath.Join(dir, "api-error.log")
	if err := os.WriteFile(outLog, []byte("one\ntwo\nthree\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(errLog, []byte("warn\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager := &fakeManager{processes: []systemd.ProcessInfo{{Name: "api", PM2Env: systemd.PM2Env{ID: 7, Name: "api", PMOutLogPath: outLog, PMErrLogPath: errLog}}}}

	result := ExecuteCommand(manager, Command{ID: "cmd-1", Action: "logs", Target: "7", Lines: 2})
	if !result.OK {
		t.Fatalf("expected command to succeed: %#v", result)
	}
	if strings.Contains(result.Message, "one") || !strings.Contains(result.Message, "two\nthree") || !strings.Contains(result.Message, "warn") {
		t.Fatalf("unexpected logs output: %s", result.Message)
	}
}
