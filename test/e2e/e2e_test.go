package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// E2ETestSuite contains all end-to-end tests
type E2ETestSuite struct {
	suite.Suite
	containerName string
	pm2goPath     string
}

// PM2Process represents a process in PM2go output
type PM2Process struct {
	PID    int    `json:"pid"`
	Name   string `json:"name"`
	PM2Env struct {
		Name        string `json:"name"`
		ExecMode    string `json:"exec_mode"`
		Status      string `json:"status"`
		PMUptime    int64  `json:"pm_uptime"`
		CreatedAt   int64  `json:"created_at"`
		RestartTime int    `json:"restart_time"`
	} `json:"pm2_env"`
	Monit struct {
		Memory int `json:"memory"`
		CPU    int `json:"cpu"`
	} `json:"monit"`
}

// SetupSuite runs before all tests
func (suite *E2ETestSuite) SetupSuite() {
	suite.containerName = "pm2go-e2e-test"
	suite.pm2goPath = "/usr/local/bin/pm2go"

	// Build pm2go binary for Linux
	suite.T().Log("Building pm2go binary for Linux...")
	buildCmd := exec.Command("go", "build", "-o", "pm2go")
	buildCmd.Dir = suite.getProjectRoot()
	buildCmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64")
	err := buildCmd.Run()
	require.NoError(suite.T(), err, "Failed to build pm2go binary")

	// Start Docker container
	suite.T().Log("Starting systemd-enabled Docker container...")
	suite.startContainer()

	// Wait for systemd to be ready
	suite.T().Log("Waiting for systemd to initialize...")
	suite.waitForSystemd()

	// Copy pm2go binary to container
	suite.T().Log("Copying pm2go binary to container...")
	suite.copyBinaryToContainer()

	// Setup pm2go in container
	suite.T().Log("Setting up pm2go in container...")
	suite.setupPM2go()
}

// TearDownSuite runs after all tests
func (suite *E2ETestSuite) TearDownSuite() {
	suite.T().Log("Cleaning up test environment...")
	
	// Stop and remove container
	exec.Command("docker", "stop", suite.containerName).Run()
	exec.Command("docker", "rm", suite.containerName).Run()
}

// SetupTest runs before each test
func (suite *E2ETestSuite) SetupTest() {
	// Clean up any existing pm2go processes by stopping systemd services
	suite.runInContainerIgnoreError("bash", "-c", "sudo -u ubuntu bash -c 'export XDG_RUNTIME_DIR=/run/user/1000 && systemctl --user stop pm2-* || true'")
	suite.runInContainerIgnoreError("bash", "-c", "sudo -u ubuntu bash -c 'export XDG_RUNTIME_DIR=/run/user/1000 && systemctl --user disable pm2-* || true'")
	suite.runInContainerIgnoreError("bash", "-c", "sudo -u ubuntu bash -c 'export XDG_RUNTIME_DIR=/run/user/1000 && systemctl --user reset-failed || true'")
	time.Sleep(2 * time.Second)
}

// Test basic PM2go operations
func (suite *E2ETestSuite) TestBasicOperations() {
	// Test 1: Start a simple application
	suite.T().Run("StartApplication", func(t *testing.T) {
		output := suite.runPM2go("start", "/home/ubuntu/apps/test-app.js", "--name", "test-basic")
		assert.Contains(t, output, "✓ Started test-basic")
	})

	// Test 2: List applications
	suite.T().Run("ListApplications", func(t *testing.T) {
		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "list")
		assert.Contains(t, output, "test-basic")
		assert.Contains(t, output, "online")
	})

	// Test 3: JSON list
	suite.T().Run("JSONList", func(t *testing.T) {
		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "jlist")
		
		var processes []PM2Process
		err := json.Unmarshal([]byte(output), &processes)
		require.NoError(t, err)
		require.Len(t, processes, 1)
		assert.Equal(t, "test-basic", processes[0].Name)
		assert.Equal(t, "online", processes[0].PM2Env.Status)
		assert.Greater(t, processes[0].PID, 0)
	})

	// Test 4: Stop application
	suite.T().Run("StopApplication", func(t *testing.T) {
		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "stop", "test-basic")
		assert.Contains(t, output, "✓ Stopped test-basic")
	})

	// Test 5: Verify stopped
	suite.T().Run("VerifyStopped", func(t *testing.T) {
		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "list")
		assert.Contains(t, output, "stopped")
	})

	// Test 6: Delete application
	suite.T().Run("DeleteApplication", func(t *testing.T) {
		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "delete", "test-basic")
		assert.Contains(t, output, "✓ Deleted test-basic")
	})
}

// Test environment variables
func (suite *E2ETestSuite) TestEnvironmentVariables() {
	// Test with command-line environment variables
	suite.T().Run("CommandLineEnvVars", func(t *testing.T) {
		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "start", "/home/ubuntu/apps/test-app.py", "--name", "test-env", "--env", "TEST_VAR=command-line-value", "--env", "PYTHON_ENV=production")
		assert.Contains(t, output, "✓ Started test-env")

		// Wait for app to start and log environment
		time.Sleep(8 * time.Second)

		// Check logs for environment variables
		logs := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "logs", "test-env", "-l", "50")
		assert.Contains(t, logs, "TEST_VAR=command-line-value")
		assert.Contains(t, logs, "PYTHON_ENV=production")

		suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "delete", "test-env")
	})
}

// Test ecosystem file functionality
func (suite *E2ETestSuite) TestEcosystemFile() {
	suite.T().Run("StartFromEcosystem", func(t *testing.T) {
		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "start", "/home/ubuntu/apps/ecosystem.json")
		assert.Contains(t, output, "✓ Started test-node-app")
		assert.Contains(t, output, "✓ Started test-python-app")
		assert.Contains(t, output, "✓ Started test-crash-app")
	})

	suite.T().Run("VerifyEcosystemApps", func(t *testing.T) {
		// Wait for apps to start
		time.Sleep(5 * time.Second)

		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "list")
		assert.Contains(t, output, "test-node-app")
		assert.Contains(t, output, "test-python-app")
		assert.Contains(t, output, "test-crash-app")
	})

	suite.T().Run("CheckEcosystemEnvVars", func(t *testing.T) {
		// Wait for logging
		time.Sleep(8 * time.Second)

		// Check Node.js app logs
		logs := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "logs", "test-node-app", "-l", "50")
		assert.Contains(t, logs, "TEST_VAR: ecosystem-node-value")
		assert.Contains(t, logs, "NODE_ENV")

		// Check Python app logs
		logs = suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "logs", "test-python-app", "-l", "50")
		assert.Contains(t, logs, "TEST_VAR=ecosystem-python-value")
		assert.Contains(t, logs, "PYTHON_ENV")
	})

	// Clean up
	suite.T().Run("CleanupEcosystem", func(t *testing.T) {
		suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "delete", "test-node-app")
		suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "delete", "test-python-app")
		suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "delete", "test-crash-app")
	})
}

// Test logs functionality
func (suite *E2ETestSuite) TestLogs() {
	// Start test application
	suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "start", "/home/ubuntu/apps/test-app.js", "--name", "test-logs")
	time.Sleep(8 * time.Second) // Wait for some log output

	suite.T().Run("ShowLogs", func(t *testing.T) {
		logs := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "logs", "test-logs", "-l", "20")
		assert.Contains(t, logs, "Test Node.js app starting")
		assert.Contains(t, logs, "Heartbeat")
	})

	suite.T().Run("ShowAllLogs", func(t *testing.T) {
		logs := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "logs", "-l", "50")
		assert.Contains(t, logs, "Test Node.js app starting")
	})

	// Clean up
	suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "delete", "test-logs")
}

// Test startup configuration
func (suite *E2ETestSuite) TestStartupConfiguration() {
	suite.T().Run("ConfigureStartup", func(t *testing.T) {
		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "startup")
		// Should already be configured in container setup
		if strings.Contains(output, "already configured") {
			assert.Contains(t, output, "✓ Startup is already configured")
		} else {
			assert.Contains(t, output, "✓ Startup configuration completed")
		}
	})
}

// Test restart and crash recovery
func (suite *E2ETestSuite) TestRestartRecovery() {
	// Start application that will crash
	suite.T().Run("StartCrashApp", func(t *testing.T) {
		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "start", "/home/ubuntu/apps/test-app.py", "--name", "crash-test", "--env", "CRASH_AFTER=15")
		assert.Contains(t, output, "✓ Started crash-test")
	})

	suite.T().Run("VerifyInitialRun", func(t *testing.T) {
		time.Sleep(5 * time.Second)
		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "list")
		assert.Contains(t, output, "crash-test")
		assert.Contains(t, output, "online")
	})

	suite.T().Run("WaitForCrashAndRestart", func(t *testing.T) {
		// Wait for crash (15 seconds) + restart delay (3 seconds) + buffer
		time.Sleep(25 * time.Second)

		// Check if service restarted (systemd should restart it)
		output := suite.runInContainer("sudo", "-u", "ubuntu", "systemctl", "--user", "status", "pm2-crash-test")
		
		// The service should have restarted at least once
		assert.Contains(t, output, "Active: active")
	})

	// Clean up
	suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "delete", "crash-test")
}

// Test flush functionality
func (suite *E2ETestSuite) TestFlush() {
	// Start app to generate logs
	suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "start", "/home/ubuntu/apps/test-app.js", "--name", "test-flush")
	time.Sleep(8 * time.Second)

	suite.T().Run("FlushSpecificApp", func(t *testing.T) {
		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "flush", "test-flush")
		assert.Contains(t, output, "✓ Flushed logs for test-flush")
	})

	suite.T().Run("FlushAllLogs", func(t *testing.T) {
		output := suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "flush")
		assert.Contains(t, output, "✓ Flushed all logs")
	})

	// Clean up
	suite.runInContainer("sudo", "-u", "ubuntu", suite.pm2goPath, "delete", "test-flush")
}

// Helper methods

func (suite *E2ETestSuite) getProjectRoot() string {
	dir, _ := os.Getwd()
	return filepath.Join(dir, "../..")
}

func (suite *E2ETestSuite) startContainer() {
	// Remove existing container
	exec.Command("docker", "stop", suite.containerName).Run()
	exec.Command("docker", "rm", suite.containerName).Run()

	// Build container
	buildCmd := exec.Command("docker", "build", "-t", "pm2go-e2e", "-f", "test/Dockerfile", ".")
	buildCmd.Dir = suite.getProjectRoot()
	err := buildCmd.Run()
	require.NoError(suite.T(), err, "Failed to build Docker image")

	// Run container with systemd
	runCmd := exec.Command("docker", "run", "-d",
		"--name", suite.containerName,
		"--privileged",
		"--cgroupns=host",
		"-v", "/sys/fs/cgroup:/sys/fs/cgroup:rw",
		"--tmpfs", "/run",
		"--tmpfs", "/run/lock",
		"--tmpfs", "/tmp",
		"pm2go-e2e")
	
	err = runCmd.Run()
	require.NoError(suite.T(), err, "Failed to start Docker container")
}

func (suite *E2ETestSuite) waitForSystemd() {
	// Wait for systemd to be ready
	for i := 0; i < 30; i++ {
		output := suite.runInContainerIgnoreError("systemctl", "is-system-running")
		if strings.Contains(output, "running") || strings.Contains(output, "degraded") {
			return
		}
		time.Sleep(2 * time.Second)
	}
	suite.T().Fatal("systemd failed to start")
}

func (suite *E2ETestSuite) copyBinaryToContainer() {
	// Copy pm2go binary to container
	copyCmd := exec.Command("docker", "cp", "pm2go", suite.containerName+":/usr/local/bin/pm2go")
	copyCmd.Dir = suite.getProjectRoot()
	err := copyCmd.Run()
	require.NoError(suite.T(), err, "Failed to copy pm2go binary")

	// Make it executable
	suite.runInContainer("chmod", "+x", "/usr/local/bin/pm2go")
}

func (suite *E2ETestSuite) setupPM2go() {
	// Initialize systemd user session using the initialization script
	suite.runInContainer("/usr/local/bin/init-user-systemd.sh")
}

func (suite *E2ETestSuite) runInContainer(args ...string) string {
	output, err := suite.runInContainerWithError(args...)
	require.NoError(suite.T(), err, "Command failed: %v\nOutput: %s", args, output)
	return output
}

func (suite *E2ETestSuite) runInContainerIgnoreError(args ...string) string {
	output, _ := suite.runInContainerWithError(args...)
	return output
}

func (suite *E2ETestSuite) runInContainerWithError(args ...string) (string, error) {
	cmd := exec.Command("docker", "exec", suite.containerName)
	cmd.Args = append(cmd.Args, args...)
	
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// Helper method to run pm2go commands as ubuntu user with proper environment
func (suite *E2ETestSuite) runPM2go(args ...string) string {
	// Build the full command with proper environment setup
	pm2goCmd := append([]string{suite.pm2goPath}, args...)
	fullCmd := fmt.Sprintf("sudo -u ubuntu bash -c 'export XDG_RUNTIME_DIR=/run/user/1000 && %s'", 
		strings.Join(pm2goCmd, " "))
	
	output := suite.runInContainer("bash", "-c", fullCmd)
	return output
}

func (suite *E2ETestSuite) runPM2goIgnoreError(args ...string) string {
	// Build the full command with proper environment setup
	pm2goCmd := append([]string{suite.pm2goPath}, args...)
	fullCmd := fmt.Sprintf("sudo -u ubuntu bash -c 'export XDG_RUNTIME_DIR=/run/user/1000 && %s || true'", 
		strings.Join(pm2goCmd, " "))
	
	output := suite.runInContainerIgnoreError("bash", "-c", fullCmd)
	return output
}

// TestE2E is the entry point for the test suite
func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping E2E tests")
	}

	suite.Run(t, new(E2ETestSuite))
}