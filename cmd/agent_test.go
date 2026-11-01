package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateAgentServiceFile(t *testing.T) {
	service := generateAgentServiceFile(agentServiceConfig{
		Executable: "/usr/local/bin/pm2go",
		EnvFile:    "/home/test/.config/pm2go/agent env",
		Interval:   30 * time.Second,
		LogFile:    "/var/log/pm2go agent.log",
	})

	checks := []string{
		"Description=PM2go outbound agent",
		"EnvironmentFile=/home/test/.config/pm2go/agent\\x20env",
		"ExecStart=\"/usr/local/bin/pm2go\" agent start --interval \"30s\"",
		"Restart=always",
		"RestartSec=5",
		"StandardOutput=append:/var/log/pm2go\\x20agent.log",
		"WantedBy=default.target",
	}

	for _, check := range checks {
		if !strings.Contains(service, check) {
			t.Fatalf("expected service file to contain %q:\n%s", check, service)
		}
	}
}

func TestGenerateAgentEnvironmentFile(t *testing.T) {
	envFile := generateAgentEnvironmentFile("https://hub.example.com", "secret with spaces", "worker-01")

	checks := []string{
		"PM2GO_API_URL=\"https://hub.example.com\"",
		"PM2GO_AGENT_SECRET=\"secret with spaces\"",
		"PM2GO_MACHINE_ID=\"worker-01\"",
	}
	for _, check := range checks {
		if !strings.Contains(envFile, check) {
			t.Fatalf("expected env file to contain %q:\n%s", check, envFile)
		}
	}
}

func TestValidateAgentAPIURLRejectsRemoteHTTP(t *testing.T) {
	if err := validateAgentAPIURL("http://hub.example.com"); err == nil {
		t.Fatal("expected remote http URL to be rejected")
	}
	if err := validateAgentAPIURL("http://127.0.0.1:9616"); err != nil {
		t.Fatalf("expected loopback http URL to be allowed: %v", err)
	}
}
