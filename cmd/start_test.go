package cmd

import (
	"reflect"
	"testing"
)

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
			"autorestart": false,
			"restart_delay": 5000
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
	if !reflect.DeepEqual([]string(ecosystem.Apps[0].Argv), []string{"--port", "3000"}) {
		t.Fatalf("expected string args to parse into an argument vector, got %#v", ecosystem.Apps[0].Argv)
	}
}

func TestParseEcosystemConfigRejectsAcceptedButUnsupportedFields(t *testing.T) {
	for _, field := range []string{"merge_logs", "combine_logs", "max_restarts"} {
		data := []byte(`{"apps":[{"name":"api","script":"app.js","` + field + `":true}]}`)
		if _, err := parseEcosystemConfig(data, ""); err == nil {
			t.Fatalf("expected %s to be rejected until its PM2 semantics are implemented", field)
		}
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

func TestParseEcosystemConfigAllowsMissingSelectedEnvironment(t *testing.T) {
	ecosystem, err := parseEcosystemConfig([]byte(`{
		"apps": [{
			"name": "api",
			"script": "app.js",
			"env": {"NODE_ENV": "development"}
		}]
	}`), "production")

	if err != nil {
		t.Fatalf("expected missing selected env to preserve base environment: %v", err)
	}
	if ecosystem.Apps[0].Env["NODE_ENV"] != "development" {
		t.Fatalf("expected base environment to remain, got %#v", ecosystem.Apps[0].Env)
	}
}

func TestParseEcosystemConfigValidatesUnselectedEnvironmentProfiles(t *testing.T) {
	_, err := parseEcosystemConfig([]byte(`{
		"apps": [{
			"name": "api",
			"script": "app.js",
			"env_production": "not-an-object"
		}]
	}`), "")
	if err == nil {
		t.Fatal("expected malformed unselected environment profile to fail")
	}
}

func TestParseEcosystemConfigRejectsNullProfilesAndTrailingContent(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"apps":[{"name":"api","script":"app.js","env_production":null}]}`),
		[]byte(`{"apps":null}`),
		[]byte(`{"apps":[]} {"apps":[]}`),
		[]byte(`{"apps":[null]}`),
		[]byte(`{"apps":[{"name":"api","script":null}]}`),
		[]byte(`{"apps":[{"name":"api"}]}`),
	} {
		if _, err := parseEcosystemConfig(data, ""); err == nil {
			t.Fatalf("expected invalid ecosystem input to fail: %s", data)
		}
	}
}

func TestParseEcosystemConfigAcceptsArrayArguments(t *testing.T) {
	ecosystem, err := parseEcosystemConfig([]byte(`{
		"apps": [{"name":"api","script":"app.js","args":["", "--name", "worker", "hello world"]}]
	}`), "")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"", "--name", "worker", "hello world"}
	if !reflect.DeepEqual([]string(ecosystem.Apps[0].Argv), expected) {
		t.Fatalf("expected %#v, got %#v", expected, ecosystem.Apps[0].Argv)
	}
}

func TestParseStartApplicationPreservesArgumentsAfterDash(t *testing.T) {
	config, err := parseStartApplication([]string{"app.js", "--name", "worker", ""}, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"--name", "worker", ""}
	if config.Script != "app.js" || !reflect.DeepEqual([]string(config.Argv), expected) {
		t.Fatalf("unexpected parsed config: %#v", config)
	}
}

func TestParseStartApplicationSupportsLegacyInterpreterSyntax(t *testing.T) {
	config, err := parseStartApplication([]string{"python3", "app.py", "--name", "worker"}, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if config.Interpreter != "python3" || config.Script != "app.py" {
		t.Fatalf("unexpected interpreter config: %#v", config)
	}
	if !reflect.DeepEqual([]string(config.Argv), []string{"--name", "worker"}) {
		t.Fatalf("unexpected arguments: %#v", config.Argv)
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
