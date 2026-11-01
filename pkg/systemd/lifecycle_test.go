package systemd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the real filesystem and command path without needing a running systemd.
func lifecycleTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	calls := filepath.Join(root, "calls")
	t.Setenv("PM2GO_TEST_CALLS", calls)
	t.Setenv("PM2GO_TEST_FAIL", "")
	t.Setenv("PM2GO_TEST_FAIL_UNIT", "")
	t.Setenv("PM2GO_TEST_STATE", "")
	bin := filepath.Join(root, "bin")
	writeLifecycleFile(t, filepath.Join(bin, "systemctl"), `#!/bin/sh
printf '%s\n' "$*" >> "$PM2GO_TEST_CALLS"
if [ "$1" = "--user" ]; then shift; fi
if [ "$1" = "$PM2GO_TEST_FAIL" ] && { [ -z "$PM2GO_TEST_FAIL_UNIT" ] || [ "$2" = "$PM2GO_TEST_FAIL_UNIT" ]; }; then
    printf 'systemd operation failed\n' >&2
    exit 1
fi
if [ "$1" = "show" ]; then
    case "$3" in
        --property=ActiveState)
            if [ -n "$PM2GO_TEST_STATE" ]; then
                printf '%s\n' "$PM2GO_TEST_STATE"
            else
                case "$2" in *stopped*) printf 'inactive\n';; *) printf 'active\n';; esac
            fi
            ;;
        *) printf '0\n';;
    esac
fi
`, 0700)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return &Manager{userMode: true, prefix: "test-", serviceDir: filepath.Join(root, "units")}, calls
}

func writeLifecycleFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func savedLifecycleUnit(t *testing.T, m *Manager, service string) string {
	t.Helper()
	path := filepath.Join(m.getServiceDir(), service+".service")
	writeLifecycleFile(t, path, "[Service]\nExecStart=/bin/sleep 100\n", 0600)
	link := filepath.Join(m.savedWantsDir(), service+".service")
	if err := ensureSymlink(link, filepath.Join("..", service+".service")); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStartAllocatesIDWithoutChangingSavedMembership(t *testing.T) {
	m, calls := lifecycleTestManager(t)
	writeLifecycleFile(t, filepath.Join(m.getServiceDir(), "test-7-existing.service"), "[Service]\nExecStart=/bin/true\n", 0600)
	if err := m.Start(AppConfig{Name: "api", Script: "/bin/sleep", Args: "100"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.getServiceDir(), "test-8-api.service")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("expected private unit with ID 8, got info=%v err=%v", info, err)
	}
	if _, err := os.Stat(m.savedWantsDir()); !os.IsNotExist(err) {
		t.Fatalf("start changed saved membership: %v", err)
	}
	output, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "--user daemon-reload\n--user start test-8-api\n" {
		t.Fatalf("unexpected start flow: %s", output)
	}
}

func TestStartFailureKeepsDefinitionForInspection(t *testing.T) {
	for _, action := range []string{"daemon-reload", "start"} {
		t.Run(action, func(t *testing.T) {
			m, _ := lifecycleTestManager(t)
			t.Setenv("PM2GO_TEST_FAIL", action)
			if err := m.Start(AppConfig{Name: "api", Script: "/bin/sleep"}); err == nil {
				t.Fatal("expected systemd failure")
			}
			if _, err := os.Stat(filepath.Join(m.getServiceDir(), "test-0-api.service")); err != nil {
				t.Fatalf("failed start lost its definition: %v", err)
			}
		})
	}
}

func TestDeletePreservesUnitWhenStopFails(t *testing.T) {
	m, _ := lifecycleTestManager(t)
	path := savedLifecycleUnit(t, m, "test-0-api")
	t.Setenv("PM2GO_TEST_FAIL", "stop")
	if err := m.Delete("api"); err == nil {
		t.Fatal("expected stop failure")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stop failure removed unit: %v", err)
	}
	if _, err := os.Readlink(filepath.Join(m.savedWantsDir(), "test-0-api.service")); err != nil {
		t.Fatalf("stop failure removed saved membership: %v", err)
	}
}

func TestDeleteReportsReloadFailureWithoutRestoringDefinitions(t *testing.T) {
	m, calls := lifecycleTestManager(t)
	path := savedLifecycleUnit(t, m, "test-0-api")
	foreign := filepath.Join(m.getServiceDir(), "other-0-worker.service.pm2go-delete-backup")
	writeLifecycleFile(t, foreign, "foreign artifact", 0600)
	t.Setenv("PM2GO_TEST_FAIL", "daemon-reload")
	if err := m.Delete("0"); err == nil || !strings.Contains(err.Error(), "reload failed") {
		t.Fatalf("expected reload failure, got %v", err)
	}
	for _, removed := range []string{path, filepath.Join(m.savedWantsDir(), "test-0-api.service")} {
		if _, err := os.Lstat(removed); !os.IsNotExist(err) {
			t.Fatalf("delete restored %s: %v", removed, err)
		}
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("delete touched another namespace: %v", err)
	}
	output, _ := os.ReadFile(calls)
	if string(output) != "--user stop test-0-api\n--user daemon-reload\n" {
		t.Fatalf("unexpected delete flow: %s", output)
	}
}

func TestDeleteAllReportsPartialSuccess(t *testing.T) {
	m, _ := lifecycleTestManager(t)
	deleted := savedLifecycleUnit(t, m, "test-0-api")
	retained := savedLifecycleUnit(t, m, "test-1-worker")
	t.Setenv("PM2GO_TEST_FAIL", "stop")
	t.Setenv("PM2GO_TEST_FAIL_UNIT", "test-1-worker")
	if err := m.Delete("all"); err == nil || !strings.Contains(err.Error(), "test-1-worker") {
		t.Fatalf("expected member failure, got %v", err)
	}
	if _, err := os.Stat(deleted); !os.IsNotExist(err) {
		t.Fatalf("successful deletion was rolled back: %v", err)
	}
	if _, err := os.Stat(retained); err != nil {
		t.Fatalf("failed stop lost its unit: %v", err)
	}
}

func TestResurrectStartsValidMembersWithoutRecreatingMissingUnits(t *testing.T) {
	m, calls := lifecycleTestManager(t)
	savedLifecycleUnit(t, m, "test-0-api")
	missing := filepath.Join(m.getServiceDir(), "test-1-missing.service")
	if err := ensureSymlink(filepath.Join(m.savedWantsDir(), "test-1-missing.service"), "../test-1-missing.service"); err != nil {
		t.Fatal(err)
	}
	if err := m.Resurrect(); err == nil || !strings.Contains(err.Error(), "test-1-missing") {
		t.Fatalf("expected missing member error, got %v", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("resurrect recreated a missing definition: %v", err)
	}
	output, _ := os.ReadFile(calls)
	if string(output) != "--user start test-0-api\n" {
		t.Fatalf("valid saved member was not started: %s", output)
	}
}

func TestSaveUpdatesNativeMembershipAndRemovesLegacyBootLinks(t *testing.T) {
	m, calls := lifecycleTestManager(t)
	for _, service := range []string{"test-0-api", "test-1-stopped"} {
		savedLifecycleUnit(t, m, service)
		if err := ensureSymlink(filepath.Join(m.getServiceDir(), "default.target.wants", service+".service"), "../"+service+".service"); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	services, err := m.savedServices()
	if err != nil || len(services) != 1 || services[0] != "test-0-api" {
		t.Fatalf("unexpected saved services: %v, err=%v", services, err)
	}
	for _, service := range []string{"test-0-api", "test-1-stopped"} {
		if _, err := os.Lstat(filepath.Join(m.getServiceDir(), "default.target.wants", service+".service")); !os.IsNotExist(err) {
			t.Fatalf("legacy boot link remains for %s: %v", service, err)
		}
	}
	target, err := os.Readlink(filepath.Join(m.getServiceDir(), "default.target.wants", m.savedTargetName()))
	if err != nil || target != "../"+m.savedTargetName() {
		t.Fatalf("unexpected boot target link: %q, err=%v", target, err)
	}
	t.Setenv("PM2GO_TEST_STATE", "inactive")
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	services, err = m.savedServices()
	if err != nil || len(services) != 0 {
		t.Fatalf("expected empty saved set, got %v, err=%v", services, err)
	}
	output, _ := os.ReadFile(calls)
	if strings.Contains(string(output), " start ") || strings.Contains(string(output), " stop ") {
		t.Fatalf("save changed runtime state: %s", output)
	}
}

func TestSaveUnknownStateLeavesMembershipUntouched(t *testing.T) {
	m, _ := lifecycleTestManager(t)
	savedLifecycleUnit(t, m, "test-0-api")
	t.Setenv("PM2GO_TEST_FAIL", "show")
	if err := m.Save(); err == nil {
		t.Fatal("expected unobservable status to fail")
	}
	services, err := m.savedServices()
	if err != nil || len(services) != 1 {
		t.Fatalf("failed observation changed membership: %v, err=%v", services, err)
	}
}

func TestSaveReloadFailureIsExplicitAndRetryable(t *testing.T) {
	m, _ := lifecycleTestManager(t)
	savedLifecycleUnit(t, m, "test-0-api")
	savedLifecycleUnit(t, m, "test-1-stopped")
	t.Setenv("PM2GO_TEST_FAIL", "daemon-reload")
	if err := m.Save(); err == nil || !strings.Contains(err.Error(), "updated on disk") {
		t.Fatalf("expected explicit partial-state error, got %v", err)
	}
	services, err := m.savedServices()
	if err != nil || len(services) != 1 || services[0] != "test-0-api" {
		t.Fatalf("membership was secretly rolled back: %v, err=%v", services, err)
	}
	t.Setenv("PM2GO_TEST_FAIL", "")
	if err := m.Save(); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
}
