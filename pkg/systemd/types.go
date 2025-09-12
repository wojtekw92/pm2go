package systemd

// AppConfig represents the configuration for a PM2 application
type AppConfig struct {
	Name   string            `json:"name"`
	Script string            `json:"script"`
	Cwd    string            `json:"cwd,omitempty"`
	Args   string            `json:"args,omitempty"`
	Env    map[string]string `json:"env,omitempty"`
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
	Name             string      `json:"name"`
	ExecMode         string      `json:"exec_mode"`
	Status           string      `json:"status"`
	PMUptime         int64       `json:"pm_uptime"`
	CreatedAt        int64       `json:"created_at"`
	RestartTime      int         `json:"restart_time"`
	UnstableRestarts int         `json:"unstable_restarts"`
	Versioning       interface{} `json:"versioning"`
	Node             PM2Node     `json:"node"`
}

type PM2Node struct {
	Version string `json:"version"`
}

type PM2Monit struct {
	Memory int `json:"memory"`
	CPU    int `json:"cpu"`
}