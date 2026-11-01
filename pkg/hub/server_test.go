package hub

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wojtekw92/pm2go/pkg/agent"
	"github.com/wojtekw92/pm2go/pkg/systemd"
)

func TestHubRequiresBearerToken(t *testing.T) {
	server := NewServer("secret")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/machines", nil)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
}

func TestDashboardDisabledByDefault(t *testing.T) {
	server := NewServer("secret")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
}

func TestDashboardEnabled(t *testing.T) {
	server := NewServer("secret")
	server.EnableDashboard()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/dashboard", nil)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("expected html content type, got %q", contentType)
	}
	if !strings.Contains(recorder.Body.String(), "PM2go Hub") {
		t.Fatalf("expected dashboard html, got %q", recorder.Body.String())
	}
}

func TestDashboardDoesNotBypassAPIAuth(t *testing.T) {
	server := NewServer("secret")
	server.EnableDashboard()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/machines", nil)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
}

func TestHubHeartbeatRegistersMachine(t *testing.T) {
	server := NewServer("secret")
	heartbeat := agent.HeartbeatRequest{
		MachineID: "worker-1",
		Hostname:  "worker-host",
		Version:   "0.1",
		Timestamp: time.Now().UTC(),
		Processes: []systemd.ProcessInfo{{Name: "api", PM2Env: systemd.PM2Env{ID: 1, Name: "api", Env: map[string]string{"SECRET": "value"}}}},
	}

	recorder := postJSON(t, server, "/api/v1/agents/heartbeat", heartbeat)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected heartbeat status 200, got %d", recorder.Code)
	}

	recorder = get(t, server, "/api/v1/machines/worker-1")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected machine status 200, got %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "SECRET") {
		t.Fatalf("expected process env to be redacted: %s", recorder.Body.String())
	}
}

func TestHubQueuesCommandsForNextHeartbeat(t *testing.T) {
	server := NewServer("secret")
	command := queueCommandRequest{Action: "restart", Target: "api"}

	recorder := postJSON(t, server, "/api/v1/machines/worker-1/commands", command)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected command status 202, got %d", recorder.Code)
	}

	recorder = postJSON(t, server, "/api/v1/agents/heartbeat", agent.HeartbeatRequest{MachineID: "worker-1"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected heartbeat status 200, got %d", recorder.Code)
	}

	var response agent.HeartbeatResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode heartbeat response: %v", err)
	}
	if len(response.Commands) != 1 || response.Commands[0].Action != "restart" || response.Commands[0].Target != "api" {
		t.Fatalf("unexpected commands: %#v", response.Commands)
	}

	recorder = postJSON(t, server, "/api/v1/agents/heartbeat", agent.HeartbeatRequest{MachineID: "worker-1"})
	var secondResponse agent.HeartbeatResponse
	if err := json.NewDecoder(recorder.Body).Decode(&secondResponse); err != nil {
		t.Fatalf("failed to decode second heartbeat response: %v", err)
	}
	if len(secondResponse.Commands) != 0 {
		t.Fatalf("expected command queue to be drained, got %#v", secondResponse.Commands)
	}

	recorder = postJSON(t, server, "/api/v1/agents/heartbeat", agent.HeartbeatRequest{
		MachineID: "worker-1",
		Results:   []agent.CommandResult{{CommandID: response.Commands[0].ID, Action: "restart", Target: "api", OK: true}},
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ack heartbeat status 200, got %d", recorder.Code)
	}

	recorder = get(t, server, "/api/v1/machines/worker-1/commands")
	if !strings.Contains(recorder.Body.String(), `"data":[]`) && !strings.Contains(recorder.Body.String(), `"data":null`) {
		t.Fatalf("expected command to be acked and removed, got %s", recorder.Body.String())
	}
}

func TestHubStoresCommandResults(t *testing.T) {
	server := NewServer("secret")
	heartbeat := agent.HeartbeatRequest{
		MachineID: "worker-1",
		Results: []agent.CommandResult{{
			CommandID: "cmd-1",
			Action:    "restart",
			Target:    "api",
			OK:        true,
			Message:   "ok",
		}},
	}

	recorder := postJSON(t, server, "/api/v1/agents/heartbeat", heartbeat)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected heartbeat status 200, got %d", recorder.Code)
	}

	recorder = get(t, server, "/api/v1/machines/worker-1/results")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected results status 200, got %d", recorder.Code)
	}
}

func postJSON(t *testing.T, server *Server, path string, payload interface{}) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer secret")
	request.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(recorder, request)
	return recorder
}

func get(t *testing.T, server *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer secret")
	server.Handler().ServeHTTP(recorder, request)
	return recorder
}
