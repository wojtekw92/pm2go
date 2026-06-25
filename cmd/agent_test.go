package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateAgentServiceFile(t *testing.T) {
	service := generateAgentServiceFile(agentServiceConfig{
		Executable: "/usr/local/bin/pm2go",
		APIURL:     "https://hub.example.com",
		Secret:     "secret with spaces",
		MachineID:  "worker-01",
		Interval:   30 * time.Second,
		LogFile:    "/var/log/pm2go agent.log",
	})

	checks := []string{
		"Description=PM2go outbound agent",
		"Environment=\"PM2GO_API_URL=https://hub.example.com\"",
		"Environment=\"PM2GO_AGENT_SECRET=secret with spaces\"",
		"Environment=\"PM2GO_MACHINE_ID=worker-01\"",
		"ExecStart=\"/usr/local/bin/pm2go\" agent start --interval \"30s\"",
		"Restart=always",
		"RestartSec=5",
		"StandardOutput=\"append:/var/log/pm2go agent.log\"",
		"WantedBy=default.target",
	}

	for _, check := range checks {
		if !strings.Contains(service, check) {
			t.Fatalf("expected service file to contain %q:\n%s", check, service)
		}
	}
}
