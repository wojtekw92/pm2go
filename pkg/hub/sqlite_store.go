package hub

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wojtekw92/pm2go/pkg/agent"
	"github.com/wojtekw92/pm2go/pkg/systemd"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &SQLiteStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) init() error {
	_, err := s.db.Exec(`
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS machines (
  machine_id TEXT PRIMARY KEY,
  hostname TEXT NOT NULL DEFAULT '',
  version TEXT NOT NULL DEFAULT '',
  last_seen TEXT NOT NULL,
  processes_json TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS commands (
  id TEXT PRIMARY KEY,
  machine_id TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  delivered_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_commands_machine_pending
  ON commands(machine_id, delivered_at, created_at);

CREATE TABLE IF NOT EXISTS command_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  machine_id TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_command_results_machine_created
  ON command_results(machine_id, created_at);
`)
	return err
}

func (s *SQLiteStore) UpdateHeartbeat(heartbeat agent.HeartbeatRequest) ([]agent.Command, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	processesJSON, err := json.Marshal(sanitizeProcesses(heartbeat.Processes))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err = tx.Exec(`
INSERT INTO machines(machine_id, hostname, version, last_seen, processes_json)
VALUES(?, ?, ?, ?, ?)
ON CONFLICT(machine_id) DO UPDATE SET
  hostname = excluded.hostname,
  version = excluded.version,
  last_seen = excluded.last_seen,
  processes_json = excluded.processes_json
`, heartbeat.MachineID, heartbeat.Hostname, heartbeat.Version, now, string(processesJSON))
	if err != nil {
		return nil, err
	}

	for _, result := range heartbeat.Results {
		payload, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`INSERT INTO command_results(machine_id, payload_json, created_at) VALUES(?, ?, ?)`, heartbeat.MachineID, string(payload), now); err != nil {
			return nil, err
		}
		if result.CommandID != "" {
			if _, err := tx.Exec(`DELETE FROM commands WHERE machine_id = ? AND id = ?`, heartbeat.MachineID, result.CommandID); err != nil {
				return nil, err
			}
		}
	}
	if _, err := tx.Exec(`
DELETE FROM command_results
WHERE machine_id = ?
  AND id NOT IN (
    SELECT id FROM command_results WHERE machine_id = ? ORDER BY created_at DESC, id DESC LIMIT ?
  )
`, heartbeat.MachineID, heartbeat.MachineID, maxStoredResults); err != nil {
		return nil, err
	}

	leaseCutoff := time.Now().UTC().Add(-commandLeaseTTL).Format(time.RFC3339Nano)
	rows, err := tx.Query(`SELECT id, payload_json FROM commands WHERE machine_id = ? AND (delivered_at IS NULL OR delivered_at < ?) ORDER BY created_at, id`, heartbeat.MachineID, leaseCutoff)
	if err != nil {
		return nil, err
	}

	var commandIDs []string
	var commands []agent.Command
	for rows.Next() {
		var id string
		var payload string
		if err := rows.Scan(&id, &payload); err != nil {
			_ = rows.Close()
			return nil, err
		}
		var command agent.Command
		if err := json.Unmarshal([]byte(payload), &command); err != nil {
			_ = rows.Close()
			return nil, err
		}
		commands = append(commands, command)
		commandIDs = append(commandIDs, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, id := range commandIDs {
		if _, err := tx.Exec(`UPDATE commands SET delivered_at = ? WHERE id = ?`, now, id); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return commands, nil
}

func (s *SQLiteStore) ListMachines() ([]Machine, error) {
	rows, err := s.db.Query(`SELECT machine_id, hostname, version, last_seen, processes_json FROM machines ORDER BY machine_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var machines []Machine
	for rows.Next() {
		machine, err := scanMachine(rows)
		if err != nil {
			return nil, err
		}
		machines = append(machines, machine)
	}
	return machines, rows.Err()
}

func (s *SQLiteStore) GetMachine(machineID string) (Machine, bool, error) {
	row := s.db.QueryRow(`SELECT machine_id, hostname, version, last_seen, processes_json FROM machines WHERE machine_id = ?`, machineID)
	machine, err := scanMachine(row)
	if err == sql.ErrNoRows {
		return Machine{}, false, nil
	}
	if err != nil {
		return Machine{}, false, err
	}
	return machine, true, nil
}

func (s *SQLiteStore) EnqueueCommand(machineID string, command agent.Command) (agent.Command, error) {
	if command.ID == "" {
		command.ID = newCommandID()
	}
	payload, err := json.Marshal(command)
	if err != nil {
		return command, err
	}

	_, err = s.db.Exec(`INSERT INTO commands(id, machine_id, payload_json, created_at) VALUES(?, ?, ?, ?)`, command.ID, machineID, string(payload), time.Now().UTC().Format(time.RFC3339Nano))
	return command, err
}

func (s *SQLiteStore) ListPendingCommands(machineID string) ([]agent.Command, error) {
	rows, err := s.db.Query(`SELECT payload_json FROM commands WHERE machine_id = ? ORDER BY created_at, id`, machineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commands []agent.Command
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var command agent.Command
		if err := json.Unmarshal([]byte(payload), &command); err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}
	return commands, rows.Err()
}

func (s *SQLiteStore) ListResults(machineID string) ([]agent.CommandResult, error) {
	rows, err := s.db.Query(`SELECT payload_json FROM command_results WHERE machine_id = ? ORDER BY created_at, id`, machineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []agent.CommandResult
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var result agent.CommandResult
		if err := json.Unmarshal([]byte(payload), &result); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

type machineScanner interface {
	Scan(dest ...interface{}) error
}

func scanMachine(scanner machineScanner) (Machine, error) {
	var machine Machine
	var lastSeen string
	var processesJSON string
	if err := scanner.Scan(&machine.MachineID, &machine.Hostname, &machine.Version, &lastSeen, &processesJSON); err != nil {
		return Machine{}, err
	}

	parsedLastSeen, err := time.Parse(time.RFC3339Nano, lastSeen)
	if err != nil {
		return Machine{}, err
	}
	machine.LastSeen = parsedLastSeen

	var processes []systemd.ProcessInfo
	if err := json.Unmarshal([]byte(processesJSON), &processes); err != nil {
		return Machine{}, err
	}
	machine.Processes = sanitizeProcesses(processes)

	return machine, nil
}

func newCommandID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("cmd-%d", time.Now().UnixNano())
	}
	return "cmd-" + hex.EncodeToString(buf)
}
