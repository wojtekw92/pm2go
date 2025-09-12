package e2e

import (
	"encoding/json"
	"flag"
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

var testDir = flag.String("testdir", "/tmp/pm2go-test", "Test directory containing pm2go binary and fixtures")

// VMTestSuite contains all VM-based end-to-end tests
type VMTestSuite struct {
	suite.Suite
	pm2goPath string
	testDir   string
}

// PM2Process struct is already declared in e2e_test.go

// SetupSuite runs before all tests
func (suite *VMTestSuite) SetupSuite() {
	flag.Parse()
	suite.testDir = *testDir
	suite.pm2goPath = filepath.Join(suite.testDir, "pm2go")

	// Verify we're in a Linux environment
	if !suite.isLinux() {
		suite.T().Skip("Skipping VM tests - not running on Linux")
	}

	// Verify systemd is available
	if !suite.hasSystemd() {
		suite.T().Skip("Skipping VM tests - systemd not available")
	}

	// Verify pm2go binary exists
	if _, err := os.Stat(suite.pm2goPath); os.IsNotExist(err) {
		suite.T().Fatalf("pm2go binary not found at %s", suite.pm2goPath)
	}

	suite.T().Logf("Running VM E2E tests with pm2go at: %s", suite.pm2goPath)
}

// SetupTest runs before each test
func (suite *VMTestSuite) SetupTest() {
	// Clean up any existing pm2go services more thoroughly
	suite.runPM2goIgnoreError("delete", "all")
	
	// Stop and disable any remaining pm2-* services
	suite.runIgnoreError("systemctl", "--user", "stop", "pm2-*")
	suite.runIgnoreError("systemctl", "--user", "disable", "pm2-*")
	
	// Reset failed services
	suite.runIgnoreError("systemctl", "--user", "reset-failed")
	
	// Reload systemd daemon
	suite.runIgnoreError("systemctl", "--user", "daemon-reload")
	
	time.Sleep(2 * time.Second)
}

// Test basic PM2go operations
func (suite *VMTestSuite) TestBasicOperations() {
	testAppJS := filepath.Join(suite.testDir, "test-app.js")

	// Test 1: Start a simple application
	suite.T().Run("StartApplication", func(t *testing.T) {
		output := suite.runPM2go("start", testAppJS, "--name", "test-basic")
		assert.Contains(t, output, "Started test-basic")
	})

	// Test 2: List applications  
	suite.T().Run("ListApplications", func(t *testing.T) {
		output := suite.runPM2go("list")
		assert.Contains(t, output, "test-basic")
		assert.Contains(t, output, "online")
	})

	// Test 3: JSON list
	suite.T().Run("JSONList", func(t *testing.T) {
		output := suite.runPM2go("jlist")
		
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
		output := suite.runPM2go("stop", "test-basic")
		assert.Contains(t, output, "Stopped test-basic")
	})

	// Test 5: Delete application
	suite.T().Run("DeleteApplication", func(t *testing.T) {
		output := suite.runPM2go("delete", "test-basic")
		assert.Contains(t, output, "Deleted test-basic")
	})
}

// Test environment variables
func (suite *VMTestSuite) TestEnvironmentVariables() {
	testAppPy := filepath.Join(suite.testDir, "test-app.py")

	// Test with command-line environment variables
	suite.T().Run("CommandLineEnvVars", func(t *testing.T) {
		output := suite.runPM2go("start", testAppPy, "--name", "test-env", "--env", "TEST_VAR=command-line-value", "--env", "PYTHON_ENV=production")
		assert.Contains(t, output, "Started test-env")

		// Wait for app to start and log environment
		time.Sleep(8 * time.Second)

		// Check logs for environment variables
		logs := suite.runPM2go("logs", "test-env", "-l", "20")
		assert.Contains(t, logs, "TEST_VAR=command-line-value")
		assert.Contains(t, logs, "PYTHON_ENV=production")

		suite.runPM2go("delete", "test-env")
	})
}

// Test ecosystem file
func (suite *VMTestSuite) TestEcosystemFile() {
	ecosystemFile := filepath.Join(suite.testDir, "ecosystem.json")

	suite.T().Run("StartFromEcosystem", func(t *testing.T) {
		output := suite.runPM2go("start", ecosystemFile)
		assert.Contains(t, output, "Started test-node-app")
		assert.Contains(t, output, "Started test-python-app") 

		// Verify all apps are running
		time.Sleep(5 * time.Second)
		list := suite.runPM2go("list")
		assert.Contains(t, list, "test-node-app")
		assert.Contains(t, list, "test-python-app")

		// Check environment variables from ecosystem
		time.Sleep(5 * time.Second)
		nodeLogs := suite.runPM2go("logs", "test-node-app", "-l", "20")
		assert.Contains(t, nodeLogs, "TEST_VAR=ecosystem-node-value")

		pythonLogs := suite.runPM2go("logs", "test-python-app", "-l", "20")
		assert.Contains(t, pythonLogs, "TEST_VAR=ecosystem-python-value")

		// Clean up
		suite.runPM2go("delete", "all")
	})
}

// Test logs functionality
func (suite *VMTestSuite) TestLogs() {
	testAppJS := filepath.Join(suite.testDir, "test-app.js")

	suite.T().Run("LogsOutput", func(t *testing.T) {
		suite.runPM2go("start", testAppJS, "--name", "test-logs")
		
		// Wait for some log output
		time.Sleep(8 * time.Second)
		
		logs := suite.runPM2go("logs", "test-logs", "-l", "10")
		assert.Contains(t, logs, "Heartbeat")
		assert.Contains(t, logs, "test-logs")

		suite.runPM2go("delete", "test-logs")
	})
}

// Helper methods
func (suite *VMTestSuite) isLinux() bool {
	cmd := exec.Command("uname")
	output, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(output)) == "Linux"
}

func (suite *VMTestSuite) hasSystemd() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func (suite *VMTestSuite) runPM2go(args ...string) string {
	output, err := suite.runPM2goWithError(args...)
	require.NoError(suite.T(), err, "pm2go command failed: %s", strings.Join(args, " "))
	return output
}

func (suite *VMTestSuite) runPM2goIgnoreError(args ...string) string {
	output, _ := suite.runPM2goWithError(args...)
	return output
}

func (suite *VMTestSuite) runPM2goWithError(args ...string) (string, error) {
	cmd := exec.Command(suite.pm2goPath, args...)
	cmd.Dir = suite.testDir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func (suite *VMTestSuite) runIgnoreError(command string, args ...string) string {
	cmd := exec.Command(command, args...)
	cmd.Dir = suite.testDir
	output, _ := cmd.CombinedOutput()
	return strings.TrimSpace(string(output))
}

// TestVM runs the VM test suite
func TestVM(t *testing.T) {
	suite.Run(t, new(VMTestSuite))
}