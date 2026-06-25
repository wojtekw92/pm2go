package cmd

import "testing"

func TestParseEcosystemConfigRejectsUnsupportedFields(t *testing.T) {
	_, err := parseEcosystemConfig([]byte(`{
		"apps": [{
			"name": "api",
			"script": "app.js",
			"instances": 2
		}]
	}`), "")

	if err == nil {
		t.Fatal("expected unsupported ecosystem field to fail parsing")
	}
}

func TestParseEcosystemConfigAcceptsSupportedFields(t *testing.T) {
	autorestart := false
	ecosystem, err := parseEcosystemConfig([]byte(`{
		"apps": [{
			"name": "api",
			"script": "app.js",
			"interpreter": "node",
			"cwd": "/srv/api",
			"args": "--port 3000",
			"env": {"NODE_ENV": "production"},
			"out_file": "logs/api.out.log",
			"error_file": "logs/api.err.log",
			"log_file": "logs/api.log",
			"merge_logs": true,
			"combine_logs": true,
			"autorestart": false,
			"restart_delay": 5000,
			"max_restarts": 3
		}]
	}`), "")

	if err != nil {
		t.Fatalf("expected supported ecosystem config to parse: %v", err)
	}
	if len(ecosystem.Apps) != 1 {
		t.Fatalf("expected one app, got %d", len(ecosystem.Apps))
	}
	if ecosystem.Apps[0].Env["NODE_ENV"] != "production" {
		t.Fatalf("expected app env to be parsed, got %#v", ecosystem.Apps[0].Env)
	}
	if ecosystem.Apps[0].LogFile != "logs/api.log" {
		t.Fatalf("expected log_file to be parsed, got %q", ecosystem.Apps[0].LogFile)
	}
	if ecosystem.Apps[0].Autorestart == nil || *ecosystem.Apps[0].Autorestart != autorestart {
		t.Fatalf("expected autorestart false, got %#v", ecosystem.Apps[0].Autorestart)
	}
	if ecosystem.Apps[0].RestartDelay != 5000 {
		t.Fatalf("expected restart_delay 5000, got %d", ecosystem.Apps[0].RestartDelay)
	}
	if ecosystem.Apps[0].MaxRestarts != 3 {
		t.Fatalf("expected max_restarts 3, got %d", ecosystem.Apps[0].MaxRestarts)
	}
}

func TestParseEcosystemConfigAppliesSelectedEnvironment(t *testing.T) {
	ecosystem, err := parseEcosystemConfig([]byte(`{
		"apps": [{
			"name": "api",
			"script": "app.js",
			"env": {"NODE_ENV": "development", "PORT": "3000"},
			"env_production": {"NODE_ENV": "production"}
		}]
	}`), "production")

	if err != nil {
		t.Fatalf("expected selected env to parse: %v", err)
	}
	if ecosystem.Apps[0].Env["NODE_ENV"] != "production" {
		t.Fatalf("expected selected env to override NODE_ENV, got %q", ecosystem.Apps[0].Env["NODE_ENV"])
	}
	if ecosystem.Apps[0].Env["PORT"] != "3000" {
		t.Fatalf("expected base env to be preserved, got %#v", ecosystem.Apps[0].Env)
	}
}

func TestParseEcosystemConfigRequiresSelectedEnvironment(t *testing.T) {
	_, err := parseEcosystemConfig([]byte(`{
		"apps": [{
			"name": "api",
			"script": "app.js",
			"env": {"NODE_ENV": "development"}
		}]
	}`), "production")

	if err == nil {
		t.Fatal("expected missing selected env to fail")
	}
}

func TestParseStartEnvFlagsSeparatesEnvironmentNameAndOverrides(t *testing.T) {
	envName, overrides, err := parseStartEnvFlags([]string{"production", "PORT=3000"})
	if err != nil {
		t.Fatalf("expected env flags to parse: %v", err)
	}
	if envName != "production" {
		t.Fatalf("expected env name production, got %q", envName)
	}
	if overrides["PORT"] != "3000" {
		t.Fatalf("expected PORT override, got %#v", overrides)
	}
}

func TestMergeEnvironmentUsesAppOverrides(t *testing.T) {
	merged := mergeEnvironment(
		map[string]string{"PATH": "/bin", "NODE_ENV": "development"},
		map[string]string{"NODE_ENV": "production", "PORT": "3000"},
	)

	if merged["PATH"] != "/bin" {
		t.Fatalf("expected base PATH to be preserved, got %q", merged["PATH"])
	}
	if merged["NODE_ENV"] != "production" {
		t.Fatalf("expected override NODE_ENV, got %q", merged["NODE_ENV"])
	}
	if merged["PORT"] != "3000" {
		t.Fatalf("expected override PORT, got %q", merged["PORT"])
	}
}
