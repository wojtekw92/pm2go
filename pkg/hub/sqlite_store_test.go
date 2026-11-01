package hub

import (
	"path/filepath"
	"testing"

	"github.com/wojtekw92/pm2go/pkg/agent"
	"github.com/wojtekw92/pm2go/pkg/systemd"
)

func TestSQLiteStorePersistsMachinesAndResults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "hub.db")
	store, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite store: %v", err)
	}

	_, err = store.UpdateHeartbeat(agent.HeartbeatRequest{
		MachineID: "worker-1",
		Hostname:  "worker-host",
		Version:   "0.1",
		Processes: []systemd.ProcessInfo{{Name: "api", PM2Env: systemd.PM2Env{ID: 1, Name: "api"}}},
		Results: []agent.CommandResult{{
			CommandID: "cmd-1",
			Action:    "restart",
			Target:    "api",
			OK:        true,
			Message:   "ok",
		}},
	})
	if err != nil {
		t.Fatalf("failed to update heartbeat: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("failed to close sqlite store: %v", err)
	}

	store, err = OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen sqlite store: %v", err)
	}
	defer store.Close()

	machine, found, err := store.GetMachine("worker-1")
	if err != nil {
		t.Fatalf("failed to get machine: %v", err)
	}
	if !found || machine.Hostname != "worker-host" || len(machine.Processes) != 1 {
		t.Fatalf("unexpected machine after reopen: found=%v machine=%#v", found, machine)
	}

	results, err := store.ListResults("worker-1")
	if err != nil {
		t.Fatalf("failed to list results: %v", err)
	}
	if len(results) != 1 || results[0].CommandID != "cmd-1" {
		t.Fatalf("unexpected results after reopen: %#v", results)
	}
}

func TestSQLiteStoreLeasesPendingCommandsUntilAck(t *testing.T) {
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "hub.db"))
	if err != nil {
		t.Fatalf("failed to open sqlite store: %v", err)
	}
	defer store.Close()

	command, err := store.EnqueueCommand("worker-1", agent.Command{Action: "restart", Target: "api"})
	if err != nil {
		t.Fatalf("failed to enqueue command: %v", err)
	}
	if command.ID == "" {
		t.Fatal("expected generated command ID")
	}

	pending, err := store.ListPendingCommands("worker-1")
	if err != nil {
		t.Fatalf("failed to list pending commands: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected one pending command, got %#v", pending)
	}

	commands, err := store.UpdateHeartbeat(agent.HeartbeatRequest{MachineID: "worker-1"})
	if err != nil {
		t.Fatalf("failed to update heartbeat: %v", err)
	}
	if len(commands) != 1 || commands[0].ID != command.ID {
		t.Fatalf("unexpected delivered commands: %#v", commands)
	}

	commands, err = store.UpdateHeartbeat(agent.HeartbeatRequest{MachineID: "worker-1"})
	if err != nil {
		t.Fatalf("failed to update second heartbeat: %v", err)
	}
	if len(commands) != 0 {
		t.Fatalf("expected command to be delivered once, got %#v", commands)
	}

	pending, err = store.ListPendingCommands("worker-1")
	if err != nil {
		t.Fatalf("failed to list pending commands after delivery: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected leased command to remain pending until ack, got %#v", pending)
	}

	_, err = store.UpdateHeartbeat(agent.HeartbeatRequest{
		MachineID: "worker-1",
		Results:   []agent.CommandResult{{CommandID: command.ID, Action: "restart", Target: "api", OK: true}},
	})
	if err != nil {
		t.Fatalf("failed to ack command result: %v", err)
	}
	pending, err = store.ListPendingCommands("worker-1")
	if err != nil {
		t.Fatalf("failed to list pending commands after ack: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending commands after ack, got %#v", pending)
	}
}
