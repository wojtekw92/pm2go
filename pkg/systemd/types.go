package systemd

// AppConfig represents the configuration for a PM2 application
type AppConfig struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	Script      string            `json:"script"`
	Interpreter string            `json:"interpreter,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	Args        string            `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

// EcosystemConfig represents PM2 ecosystem file structure
type EcosystemConfig struct {
	Apps []AppConfig `json:"apps"`
}

// ProcessInfo represents PM2-compatible process information for JSON output
type ProcessInfo struct {
	PID    int     `json:"pid"`
	Name   string  `json:"name"`
	PM2Env PM2Env  `json:"pm2_env"`
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
	Node             PM2Node           `json:"node"`
	PMExecPath       string            `json:"pm_exec_path"`
	PMOutLogPath     string            `json:"pm_out_log_path"`
	PMErrLogPath     string            `json:"pm_err_log_path"`
	PMPidPath        string            `json:"pm_pid_path"`
	Interpreter      string            `json:"interpreter"`
	Args             string            `json:"args"`
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
	Script      string
	Interpreter string
	Args        string
	OutLogPath  string
	ErrLogPath  string
	PidPath     string
	Env         map[string]string
}