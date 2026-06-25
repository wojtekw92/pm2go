package systemd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitCommandLinePreservesQuotedArguments(t *testing.T) {
	args := splitCommandLine("--interval 1 --message 'App 1 output' --flag=quoted\\ value")
	expected := []string{"--interval", "1", "--message", "App 1 output", "--flag=quoted value"}

	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %#v", len(expected), len(args), args)
	}

	for i := range expected {
		if args[i] != expected[i] {
			t.Fatalf("arg %d: expected %q, got %q", i, expected[i], args[i])
		}
	}
}

func TestQuoteSystemdValueEscapesSpecialValues(t *testing.T) {
	quoted := quoteSystemdValue("value with spaces % and \"quotes\"")
	if quoted != "\"value with spaces %% and \\\"quotes\\\"\"" {
		t.Fatalf("unexpected quoted value: %s", quoted)
	}

	unquoted := unquoteSystemdValue(quoted)
	if unquoted != "value with spaces % and \"quotes\"" {
		t.Fatalf("unexpected unquoted value: %s", unquoted)
	}
}

func TestServiceNameEscapesAndRestoresUnicodeName(t *testing.T) {
	manager := NewManager()
	serviceName := manager.serviceNameWithID(7, "test-café app")

	if strings.Contains(serviceName, "é") || strings.Contains(serviceName, " ") {
		t.Fatalf("expected escaped service name, got %q", serviceName)
	}
	if !strings.Contains(serviceName, `caf\xc3\xa9\x20app`) {
		t.Fatalf("expected escaped unicode and space in service name, got %q", serviceName)
	}

	id, appName, err := manager.parseServiceName(serviceName)
	if err != nil {
		t.Fatal(err)
	}
	if id != 7 || appName != "test-café app" {
		t.Fatalf("expected id/name restored, got %d/%q", id, appName)
	}
}

func TestGenerateServiceFileQuotesExecStartAndEnvironment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	manager := NewManager()

	service := manager.generateServiceFile(AppConfig{
		Name:        "test app",
		Script:      "script with spaces.py",
		Interpreter: "python3",
		Cwd:         "/tmp/path with spaces",
		Args:        "--message 'hello world'",
		Env: map[string]string{
			"SPECIAL": "value with spaces % and \"quotes\"",
		},
	})

	checks := []string{
		"WorkingDirectory=/tmp/path\\x20with\\x20spaces",
		"\"script with spaces.py\"",
		"\"hello world\"",
		"Environment=\"SPECIAL=value with spaces %% and \\\"quotes\\\"\"",
	}

	for _, check := range checks {
		if !strings.Contains(service, check) {
			t.Fatalf("expected service file to contain %q:\n%s", check, service)
		}
	}
}

func TestGenerateServiceFileMapsCustomLogsAndRestartOptions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	manager := NewManager()
	autorestart := false

	service := manager.generateServiceFile(AppConfig{
		Name:         "api",
		Script:       "app.js",
		Cwd:          "/srv/api",
		LogFile:      "logs/api.log",
		Autorestart:  &autorestart,
		RestartDelay: 1500,
		MaxRestarts:  4,
	})

	checks := []string{
		"StartLimitIntervalSec=60",
		"StartLimitBurst=4",
		"Restart=no",
		"RestartSec=1500ms",
		"StandardOutput=append:/srv/api/logs/api.log",
		"StandardError=append:/srv/api/logs/api.log",
	}

	for _, check := range checks {
		if !strings.Contains(service, check) {
			t.Fatalf("expected service file to contain %q:\n%s", check, service)
		}
	}
}

func TestGenerateServiceFileMapsSeparateCustomLogs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	manager := NewManager()

	service := manager.generateServiceFile(AppConfig{
		Name:      "api",
		Script:    "app.js",
		Cwd:       "/srv/api",
		OutFile:   "/var/log/api/out.log",
		ErrorFile: "NULL",
	})

	checks := []string{
		"Restart=always",
		"RestartSec=3",
		"StandardOutput=append:/var/log/api/out.log",
		"StandardError=null",
	}

	for _, check := range checks {
		if !strings.Contains(service, check) {
			t.Fatalf("expected service file to contain %q:\n%s", check, service)
		}
	}
}

func TestGenerateServiceFileMakesRelativeWorkingDirectoryAbsolute(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	manager := NewManager()

	service := manager.generateServiceFile(AppConfig{
		Name:   "api",
		Script: "app.js",
		Cwd:    "test/fixtures",
	})

	expected, err := filepath.Abs("test/fixtures")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(service, "WorkingDirectory="+systemdPathValue(expected)) {
		t.Fatalf("expected absolute working directory %q:\n%s", expected, service)
	}
}

func TestSystemdDurationParsing(t *testing.T) {
	tests := map[string]int{
		"1500ms": 1500,
		"1.5s":   1500,
		"3":      3000,
	}

	for input, expected := range tests {
		if actual := parseSystemdDurationMillis(input); actual != expected {
			t.Fatalf("parseSystemdDurationMillis(%q): expected %d, got %d", input, expected, actual)
		}
	}
}
