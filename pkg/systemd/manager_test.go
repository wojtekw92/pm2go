package systemd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitCommandLinePreservesQuotedArguments(t *testing.T) {
	args := splitCommandLine("'' --interval 1 --message 'App 1 output' --flag=quoted\\ value")
	expected := []string{"", "--interval", "1", "--message", "App 1 output", "--flag=quoted value"}

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
		Args:        "--message 'hello world' '[Install]'",
		Env: map[string]string{
			"SPECIAL": "value with spaces % and \"quotes\"",
		},
	})

	checks := []string{
		"WorkingDirectory=/tmp/path\\x20with\\x20spaces",
		"\"script with spaces.py\"",
		"\"hello world\"",
		"\"[Install]\"",
		"Environment=\"SPECIAL=value with spaces %% and \\\"quotes\\\"\"",
	}

	for _, check := range checks {
		if !strings.Contains(service, check) {
			t.Fatalf("expected service file to contain %q:\n%s", check, service)
		}
	}
}

func TestGenerateServiceFilePreservesEmptyAndFlagArguments(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	manager := NewManager()

	service := manager.generateServiceFile(AppConfig{
		Name:      "api",
		Script:    "app.js",
		Argv:      AppArgs{"", "--name", "worker", "hello world"},
		CreatedAt: 1234,
	})

	checks := []string{`""`, `"--name"`, `"worker"`, `"hello world"`, "# PM2Go-CreatedAt=1234"}
	for _, check := range checks {
		if !strings.Contains(service, check) {
			t.Fatalf("expected service file to contain %q:\n%s", check, service)
		}
	}
}

func TestReadServiceConfigUsesGeneratedMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	manager := NewManager()
	serviceDir := manager.getServiceDir()
	if err := os.MkdirAll(serviceDir, 0700); err != nil {
		t.Fatal(err)
	}

	config := AppConfig{
		Name:        "api",
		Script:      "app.js",
		Interpreter: "node --trace-warnings",
		Argv:        AppArgs{"", "--name", "worker"},
		CreatedAt:   1700000000000,
	}
	serviceName := manager.serviceNameWithID(3, config.Name)
	servicePath := filepath.Join(serviceDir, serviceName+".service")
	if err := os.WriteFile(servicePath, []byte(manager.generateServiceFile(config)), 0600); err != nil {
		t.Fatal(err)
	}

	parsed := manager.readServiceConfig(serviceName)
	if parsed.Script != config.Script || parsed.Interpreter != config.Interpreter {
		t.Fatalf("metadata did not preserve execution config: %#v", parsed)
	}
	if parsed.CreatedAt != config.CreatedAt {
		t.Fatalf("expected created_at %d, got %d", config.CreatedAt, parsed.CreatedAt)
	}
	if len(parsed.Args) != 3 || parsed.Args[0] != "" || parsed.Args[1] != "--name" || parsed.Args[2] != "worker" {
		t.Fatalf("metadata did not preserve arguments: %#v", parsed.Args)
	}
}

func TestSavedTargetUsesSystemdWantsLinks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	manager := NewManager()
	serviceDir := manager.getServiceDir()
	if err := os.MkdirAll(serviceDir, 0700); err != nil {
		t.Fatal(err)
	}

	serviceName := manager.serviceNameWithID(0, "api")
	if err := os.WriteFile(filepath.Join(serviceDir, serviceName+".service"), []byte("[Service]\nExecStart=/bin/true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(manager.savedWantsDir(), 0755); err != nil {
		t.Fatal(err)
	}
	linkName := serviceName + ".service"
	if err := os.Symlink(filepath.Join("..", linkName), filepath.Join(manager.savedWantsDir(), linkName)); err != nil {
		t.Fatal(err)
	}

	services, err := manager.savedServices()
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 || services[0] != serviceName {
		t.Fatalf("unexpected saved services: %#v", services)
	}
}

func TestGeneratedServiceJoinsSavedTarget(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	manager := NewManager()
	service := manager.generateServiceFile(AppConfig{Name: "api", Script: "app.js"})
	if !strings.Contains(service, "WantedBy="+manager.savedTargetName()) {
		t.Fatalf("expected service to join saved target:\n%s", service)
	}
}

func TestUserServiceDirUsesXDGConfigHome(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	manager := NewManager()
	if manager.userMode {
		expected := filepath.Join(configHome, "systemd/user")
		if manager.getServiceDir() != expected {
			t.Fatalf("expected %s, got %s", expected, manager.getServiceDir())
		}
	}
}

func TestManagerRejectsUnsafeServicePrefix(t *testing.T) {
	t.Setenv("PM2GO_SERVICE_PREFIX", "../escape-")
	manager := NewManager()
	if _, err := manager.List(); err == nil {
		t.Fatal("expected unsafe service prefix to be rejected")
	}
}

func TestSavedServicesReturnsValidMembersWithCorruptionError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	manager := NewManager()
	if err := os.MkdirAll(manager.savedWantsDir(), 0755); err != nil {
		t.Fatal(err)
	}
	serviceName := manager.serviceNameWithID(0, "api")
	name := serviceName + ".service"
	if err := os.WriteFile(filepath.Join(manager.getServiceDir(), name), []byte("[Service]\nExecStart=/bin/true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", name), filepath.Join(manager.savedWantsDir(), name)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../missing.service", filepath.Join(manager.savedWantsDir(), manager.prefix+"1-missing.service")); err != nil {
		t.Fatal(err)
	}

	services, err := manager.savedServices()
	if err == nil {
		t.Fatal("expected dangling membership to be reported")
	}
	if len(services) != 1 || services[0] != serviceName {
		t.Fatalf("expected valid member to remain usable, got %#v", services)
	}
}

func TestMapSystemdStatusIncludesFailedState(t *testing.T) {
	manager := NewManager()
	if status := manager.mapSystemdStatus("failed"); status != "errored" {
		t.Fatalf("expected failed service to be errored, got %q", status)
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
		"RestartSec=0",
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
