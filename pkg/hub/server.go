package hub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wojtekw92/pm2go/pkg/agent"
	"github.com/wojtekw92/pm2go/pkg/systemd"
)

type Server struct {
	token     string
	store     Store
	dashboard bool
}

const (
	maxHeartbeatBytes = 1 << 20
	maxCommandBytes   = 64 << 10
	commandLeaseTTL   = time.Minute
	maxStoredResults  = 200
)

type Store interface {
	UpdateHeartbeat(agent.HeartbeatRequest) ([]agent.Command, error)
	ListMachines() ([]Machine, error)
	GetMachine(string) (Machine, bool, error)
	EnqueueCommand(string, agent.Command) (agent.Command, error)
	ListPendingCommands(string) ([]agent.Command, error)
	ListResults(string) ([]agent.CommandResult, error)
}

type MemoryStore struct {
	mu       sync.Mutex
	machines map[string]*Machine
	commands map[string][]queuedCommand
	results  map[string][]agent.CommandResult
	nextID   int64
}

type queuedCommand struct {
	command  agent.Command
	leasedAt time.Time
}

type Machine struct {
	MachineID string                `json:"machine_id"`
	Hostname  string                `json:"hostname"`
	Version   string                `json:"version"`
	LastSeen  time.Time             `json:"last_seen"`
	Processes []systemd.ProcessInfo `json:"processes"`
}

type queueCommandRequest struct {
	ID     string             `json:"id,omitempty"`
	Action string             `json:"action"`
	Target string             `json:"target,omitempty"`
	Signal string             `json:"signal,omitempty"`
	Lines  int                `json:"lines,omitempty"`
	App    *systemd.AppConfig `json:"app,omitempty"`
}

type response struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func NewServer(token string) *Server {
	return NewServerWithStore(token, NewMemoryStore())
}

func NewServerWithStore(token string, store Store) *Server {
	return &Server{token: token, store: store}
}

func (s *Server) EnableDashboard() {
	s.dashboard = true
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		machines: make(map[string]*Machine),
		commands: make(map[string][]queuedCommand),
		results:  make(map[string][]agent.CommandResult),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	if s.dashboard {
		mux.HandleFunc("/dashboard", s.handleDashboard)
	}
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/v1/agents/heartbeat", s.handleHeartbeat)
	mux.HandleFunc("/api/v1/machines", s.handleMachines)
	mux.HandleFunc("/api/v1/machines/", s.handleMachine)
	return s.auth(mux)
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dashboard" && r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		if s.token == "" {
			next.ServeHTTP(w, r)
			return
		}

		if r.Header.Get("Authorization") != "Bearer "+s.token {
			writeError(w, http.StatusUnauthorized, "missing or invalid bearer token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(dashboardHTML))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, response{OK: true, Data: map[string]string{"status": "ok"}})
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var heartbeat agent.HeartbeatRequest
	if err := decodeJSON(w, r, maxHeartbeatBytes, &heartbeat); err != nil {
		writeError(w, http.StatusBadRequest, "invalid heartbeat: "+err.Error())
		return
	}
	if heartbeat.MachineID == "" {
		writeError(w, http.StatusBadRequest, "machine_id is required")
		return
	}

	commands, err := s.store.UpdateHeartbeat(heartbeat)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agent.HeartbeatResponse{Commands: commands})
}

func (s *Server) handleMachines(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	machines, err := s.store.ListMachines()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response{OK: true, Data: machines})
}

func (s *Server) handleMachine(w http.ResponseWriter, r *http.Request) {
	machineID, action, ok := parseMachinePath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown machine endpoint")
		return
	}

	switch action {
	case "":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		machine, found, err := s.store.GetMachine(machineID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, fmt.Sprintf("machine %q not found", machineID))
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: machine})
	case "commands":
		s.handleMachineCommands(w, r, machineID)
	case "results":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		results, err := s.store.ListResults(machineID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: results})
	default:
		writeError(w, http.StatusNotFound, "unknown machine action")
	}
}

func (s *Server) handleMachineCommands(w http.ResponseWriter, r *http.Request, machineID string) {
	switch r.Method {
	case http.MethodGet:
		commands, err := s.store.ListPendingCommands(machineID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, response{OK: true, Data: commands})
	case http.MethodPost:
		var req queueCommandRequest
		if err := decodeJSON(w, r, maxCommandBytes, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid command: "+err.Error())
			return
		}
		if req.Action == "" {
			writeError(w, http.StatusBadRequest, "action is required")
			return
		}

		command := agent.Command{
			ID:     req.ID,
			Action: req.Action,
			Target: req.Target,
			Signal: req.Signal,
			Lines:  req.Lines,
			App:    req.App,
		}
		command, err := s.store.EnqueueCommand(machineID, command)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, response{OK: true, Message: "command queued", Data: command})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *MemoryStore) UpdateHeartbeat(heartbeat agent.HeartbeatRequest) ([]agent.Command, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.machines[heartbeat.MachineID] = &Machine{
		MachineID: heartbeat.MachineID,
		Hostname:  heartbeat.Hostname,
		Version:   heartbeat.Version,
		LastSeen:  time.Now().UTC(),
		Processes: sanitizeProcesses(heartbeat.Processes),
	}

	if len(heartbeat.Results) > 0 {
		s.results[heartbeat.MachineID] = append(s.results[heartbeat.MachineID], heartbeat.Results...)
		s.results[heartbeat.MachineID] = trimResults(s.results[heartbeat.MachineID])
		s.ackResults(heartbeat.MachineID, heartbeat.Results)
	}

	now := time.Now().UTC()
	commands := []agent.Command{}
	queue := s.commands[heartbeat.MachineID]
	for i := range queue {
		if queue[i].leasedAt.IsZero() || now.Sub(queue[i].leasedAt) > commandLeaseTTL {
			commands = append(commands, queue[i].command)
			queue[i].leasedAt = now
		}
	}
	s.commands[heartbeat.MachineID] = queue
	return commands, nil
}

func (s *MemoryStore) ackResults(machineID string, results []agent.CommandResult) {
	if len(results) == 0 {
		return
	}
	acked := make(map[string]bool, len(results))
	for _, result := range results {
		if result.CommandID != "" {
			acked[result.CommandID] = true
		}
	}
	if len(acked) == 0 {
		return
	}
	queue := s.commands[machineID]
	kept := queue[:0]
	for _, item := range queue {
		if !acked[item.command.ID] {
			kept = append(kept, item)
		}
	}
	s.commands[machineID] = kept
}

func (s *MemoryStore) ListMachines() ([]Machine, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	machines := make([]Machine, 0, len(s.machines))
	for _, machine := range s.machines {
		machines = append(machines, *machine)
	}
	return machines, nil
}

func (s *MemoryStore) GetMachine(machineID string) (Machine, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	machine, ok := s.machines[machineID]
	if !ok {
		return Machine{}, false, nil
	}
	return *machine, true, nil
}

func (s *MemoryStore) EnqueueCommand(machineID string, command agent.Command) (agent.Command, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if command.ID == "" {
		s.nextID++
		command.ID = fmt.Sprintf("cmd-%d", s.nextID)
	}
	s.commands[machineID] = append(s.commands[machineID], queuedCommand{command: command})
	return command, nil
}

func (s *MemoryStore) ListPendingCommands(machineID string) ([]agent.Command, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	queue := s.commands[machineID]
	commands := make([]agent.Command, 0, len(queue))
	for _, item := range queue {
		commands = append(commands, item.command)
	}
	return commands, nil
}

func (s *MemoryStore) ListResults(machineID string) ([]agent.CommandResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]agent.CommandResult(nil), s.results[machineID]...), nil
}

func sanitizeProcesses(processes []systemd.ProcessInfo) []systemd.ProcessInfo {
	sanitized := make([]systemd.ProcessInfo, len(processes))
	copy(sanitized, processes)
	for i := range sanitized {
		sanitized[i].PM2Env.Env = nil
	}
	return sanitized
}

func trimResults(results []agent.CommandResult) []agent.CommandResult {
	if len(results) <= maxStoredResults {
		return results
	}
	return append([]agent.CommandResult(nil), results[len(results)-maxStoredResults:]...)
}

func parseMachinePath(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, "/api/v1/machines/")
	if rest == path || rest == "" {
		return "", "", false
	}

	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 1 {
		return parts[0], "", true
	}
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return "", "", false
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	if status == http.StatusRequestEntityTooLarge {
		message = "request body too large"
	}
	writeJSON(w, status, response{OK: false, Message: message})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, target interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("request body must contain one JSON value")
	}
	return nil
}
