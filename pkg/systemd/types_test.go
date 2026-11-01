package systemd

import (
	"encoding/json"
	"testing"
)

func TestProcessInfoUsesPM2JSONFieldNames(t *testing.T) {
	process := ProcessInfo{
		PID:  42,
		Name: "api",
		PMID: 3,
		PM2Env: PM2Env{
			ID:          3,
			Name:        "api",
			ExecMode:    "fork_mode",
			PMUptime:    1700000000000,
			Interpreter: "node",
			Args:        AppArgs{"--name", "worker"},
		},
	}

	data, err := json.Marshal(process)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["pm_id"] != float64(3) {
		t.Fatalf("expected top-level pm_id, got %s", data)
	}
	env := decoded["pm2_env"].(map[string]interface{})
	if env["exec_mode"] != "fork_mode" || env["exec_interpreter"] != "node" {
		t.Fatalf("expected PM2 execution fields, got %s", data)
	}
	args := env["args"].([]interface{})
	if len(args) != 2 || args[0] != "--name" || args[1] != "worker" {
		t.Fatalf("expected argument array, got %s", data)
	}
}
