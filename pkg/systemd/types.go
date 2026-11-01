package systemd

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AppArgs accepts both PM2 ecosystem argument forms while keeping arguments
// structured internally so quoting and empty values survive CLI processing.
type AppArgs []string

func (a *AppArgs) UnmarshalJSON(data []byte) error {
	var values []string
	if err := json.Unmarshal(data, &values); err == nil {
		*a = values
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*a = splitCommandLine(value)
		return nil
	}

	return fmt.Errorf("args must be a string or an array of strings")
}

// AppConfig represents the configuration for a PM2 application
type AppConfig struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Script       string            `json:"script"`
	Interpreter  string            `json:"interpreter,omitempty"`
	Cwd          string            `json:"cwd,omitempty"`
	Args         string            `json:"args,omitempty"`
	Argv         AppArgs           `json:"-"`
	Env          map[string]string `json:"env,omitempty"`
	OutFile      string            `json:"out_file,omitempty"`
	ErrorFile    string            `json:"error_file,omitempty"`
	LogFile      string            `json:"log_file,omitempty"`
	MergeLogs    bool              `json:"merge_logs,omitempty"`
	CombineLogs  bool              `json:"combine_logs,omitempty"`
	Autorestart  *bool             `json:"autorestart,omitempty"`
	RestartDelay int               `json:"restart_delay,omitempty"`
	MaxRestarts  int               `json:"max_restarts,omitempty"`
	CreatedAt    int64             `json:"-"`
}

func (c *AppConfig) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	argsData := fields["args"]
	delete(fields, "args")
	withoutArgs, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	type appConfigAlias AppConfig
	if err := json.Unmarshal(withoutArgs, (*appConfigAlias)(c)); err != nil {
		return err
	}
	if len(argsData) == 0 {
		return nil
	}

	var args AppArgs
	if err := json.Unmarshal(argsData, &args); err != nil {
		return err
	}
	c.Argv = args
	var stringArgs string
	if err := json.Unmarshal(argsData, &stringArgs); err == nil {
		c.Args = stringArgs
	} else {
		c.Args = strings.Join(args, " ")
	}
	return nil
}

func (c AppConfig) MarshalJSON() ([]byte, error) {
	type appConfigAlias AppConfig
	data, err := json.Marshal(appConfigAlias(c))
	if err != nil {
		return nil, err
	}
	if c.Argv == nil {
		return data, nil
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	fields["args"] = []string(c.Argv)
	return json.Marshal(fields)
}

// EcosystemConfig represents PM2 ecosystem file structure
type EcosystemConfig struct {
	Apps []AppConfig `json:"apps"`
}

// ProcessInfo represents PM2-compatible process information for JSON output
type ProcessInfo struct {
	PID    int      `json:"pid"`
	Name   string   `json:"name"`
	PMID   int      `json:"pm_id"`
	PM2Env PM2Env   `json:"pm2_env"`
	Monit  PM2Monit `json:"monit"`
}

type PM2Env struct {
	ID               int               `json:"pm_id"`
	Name             string            `json:"name"`
	ExecMode         string            `json:"exec_mode"`
	Status           string            `json:"status"`
	PMUptime         int64             `json:"pm_uptime"`
	CreatedAt        int64             `json:"created_at"`
	RestartTime      int               `json:"restart_time"`
	UnstableRestarts int               `json:"unstable_restarts"`
	Versioning       interface{}       `json:"versioning"`
	Node             PM2Node           `json:"-"`
	NodeVersion      string            `json:"node_version,omitempty"`
	PMExecPath       string            `json:"pm_exec_path"`
	PMCwd            string            `json:"pm_cwd"`
	PMOutLogPath     string            `json:"pm_out_log_path"`
	PMErrLogPath     string            `json:"pm_err_log_path"`
	PMPidPath        string            `json:"pm_pid_path"`
	Interpreter      string            `json:"exec_interpreter"`
	Args             AppArgs           `json:"args"`
	Env              map[string]string `json:"env"`
}

type PM2Node struct {
	Version string `json:"version"`
}

type PM2Monit struct {
	Memory int `json:"memory"`
	CPU    int `json:"cpu"`
}

// ServiceConfig holds parsed service file configuration
type ServiceConfig struct {
	Script       string
	Interpreter  string
	Cwd          string
	Args         AppArgs
	OutLogPath   string
	ErrLogPath   string
	PidPath      string
	Env          map[string]string
	Autorestart  bool
	RestartDelay int
	MaxRestarts  int
	CreatedAt    int64
}
