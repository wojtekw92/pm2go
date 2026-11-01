package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wojtekw92/pm2go/pkg/systemd"
)

var startCmd = &cobra.Command{
	Use:                "start <script|ecosystem.json> [-- app arguments...]",
	Short:              "Start an application or ecosystem",
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		envVars, _ := cmd.Flags().GetStringArray("env")
		interpreter, _ := cmd.Flags().GetString("interpreter")
		cwd, _ := cmd.Flags().GetString("cwd")
		outFile, _ := cmd.Flags().GetString("output")
		errorFile, _ := cmd.Flags().GetString("error")
		logFile, _ := cmd.Flags().GetString("log")

		handleStart(args, cmd.ArgsLenAtDash(), systemd.AppConfig{
			Name:        name,
			Interpreter: interpreter,
			Cwd:         cwd,
			OutFile:     outFile,
			ErrorFile:   errorFile,
			LogFile:     logFile,
		}, envVars)
	},
}

func init() {
	startCmd.Flags().StringP("name", "n", "", "Application name")
	startCmd.Flags().StringArray("env", []string{}, "Environment name or variable override (KEY=VALUE)")
	startCmd.Flags().String("interpreter", "", "Interpreter used to run the application")
	startCmd.Flags().String("cwd", "", "Working directory")
	startCmd.Flags().StringP("output", "o", "", "Standard output log path")
	startCmd.Flags().StringP("error", "e", "", "Standard error log path")
	startCmd.Flags().StringP("log", "l", "", "Combined output and error log path")
}

func handleStart(args []string, argsLenAtDash int, config systemd.AppConfig, envVars []string) {
	// Check if we're restarting an existing process by ID
	if len(args) == 1 {
		if _, err := os.Stat(args[0]); err != nil {
			if id, err := strconv.Atoi(args[0]); err == nil {
				handleStartRestart(id)
				return
			}
		}

		if isEcosystemFile(args[0]) {
			handleEcosystemStart(args[0], envVars)
			return
		}
	}

	parsed, err := parseStartApplication(args, argsLenAtDash, config.Interpreter)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	config.Script = parsed.Script
	config.Interpreter = parsed.Interpreter
	config.Argv = parsed.Argv
	config.Env = currentEnvironment()

	// Add/override with command-line environment variables
	for _, envVar := range envVars {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			config.Env[parts[0]] = parts[1]
		} else {
			fmt.Printf("Warning: ignoring ecosystem environment %q for a direct start\n", envVar)
		}
	}

	if config.Name == "" {
		config.Name = strings.TrimSuffix(filepath.Base(config.Script), filepath.Ext(config.Script))
	}

	if err := manager.Start(config); err != nil {
		fmt.Printf("Error starting %s: %v\n", config.Name, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Started %s\n", config.Name)
}

func isEcosystemFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".json")
}

func parseStartApplication(args []string, argsLenAtDash int, interpreter string) (systemd.AppConfig, error) {
	if len(args) == 0 {
		return systemd.AppConfig{}, fmt.Errorf("script is required")
	}

	config := systemd.AppConfig{Interpreter: interpreter}
	if argsLenAtDash < 0 {
		config.Script = args[0]
		config.Argv = append(systemd.AppArgs(nil), args[1:]...)
		return config, nil
	}
	if argsLenAtDash == 0 || argsLenAtDash > len(args) {
		return systemd.AppConfig{}, fmt.Errorf("script is required before '--'")
	}

	beforeDash := args[:argsLenAtDash]
	afterDash := args[argsLenAtDash:]
	if interpreter == "" && len(beforeDash) == 1 && len(afterDash) > 0 && isLegacyInterpreter(beforeDash[0]) && !strings.HasPrefix(afterDash[0], "-") {
		config.Interpreter = beforeDash[0]
		config.Script = afterDash[0]
		config.Argv = append(systemd.AppArgs(nil), afterDash[1:]...)
		return config, nil
	}

	config.Script = beforeDash[0]
	config.Argv = append(systemd.AppArgs(nil), beforeDash[1:]...)
	config.Argv = append(config.Argv, afterDash...)
	return config, nil
}

func isLegacyInterpreter(value string) bool {
	switch filepath.Base(value) {
	case "node", "nodejs", "python", "python2", "python3", "ruby", "bash", "sh", "zsh", "perl", "php":
		return true
	default:
		return false
	}
}

func handleStartRestart(id int) {
	// Find the process by ID to get its name
	processes, err := manager.List()
	if err != nil {
		fmt.Printf("Error getting process list: %v\n", err)
		os.Exit(1)
	}

	var targetProcess *systemd.ProcessInfo
	for i, process := range processes {
		if process.PM2Env.ID == id {
			targetProcess = &processes[i]
			break
		}
	}

	if targetProcess == nil {
		fmt.Printf("Error: Process with ID %d not found\n", id)
		os.Exit(1)
	}

	// Check if it's already running
	if targetProcess.PM2Env.Status == "online" {
		fmt.Printf("Process %d (%s) is already running\n", id, targetProcess.Name)
		return
	}

	// Restart the service
	if err := manager.Restart(id); err != nil {
		fmt.Printf("Error restarting %s: %v\n", targetProcess.Name, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Restarted %s (ID: %d)\n", targetProcess.Name, id)
}

func handleEcosystemStart(filename string, envVars []string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading ecosystem file: %v\n", err)
		os.Exit(1)
	}

	envName, envOverrides, err := parseStartEnvFlags(envVars)
	if err != nil {
		fmt.Printf("Error parsing --env flags: %v\n", err)
		os.Exit(1)
	}

	ecosystem, err := parseEcosystemConfig(data, envName)
	if err != nil {
		fmt.Printf("Error parsing ecosystem file: %v\n", err)
		os.Exit(1)
	}

	failed := false
	for _, app := range ecosystem.Apps {
		app.Env = mergeEnvironment(currentEnvironment(), app.Env)
		app.Env = mergeEnvironment(app.Env, envOverrides)
		if app.Name == "" {
			app.Name = strings.TrimSuffix(filepath.Base(app.Script), filepath.Ext(app.Script))
		}
		if err := manager.Start(app); err != nil {
			fmt.Printf("Error starting %s: %v\n", app.Name, err)
			failed = true
		} else {
			fmt.Printf("✓ Started %s\n", app.Name)
		}
	}
	if failed {
		os.Exit(1)
	}
}

func parseEcosystemConfig(data []byte, envName string) (systemd.EcosystemConfig, error) {
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return systemd.EcosystemConfig{}, fmt.Errorf("unsupported or invalid ecosystem option: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return systemd.EcosystemConfig{}, fmt.Errorf("invalid trailing ecosystem content")
	}

	for key := range raw {
		if key != "apps" {
			return systemd.EcosystemConfig{}, fmt.Errorf("unsupported ecosystem field %q", key)
		}
	}

	appsData, ok := raw["apps"]
	if !ok || bytes.Equal(bytes.TrimSpace(appsData), []byte("null")) {
		return systemd.EcosystemConfig{}, fmt.Errorf("apps must be an array")
	}
	var rawApps []json.RawMessage
	if err := json.Unmarshal(appsData, &rawApps); err != nil {
		return systemd.EcosystemConfig{}, fmt.Errorf("invalid apps field: %w", err)
	}

	ecosystem := systemd.EcosystemConfig{Apps: make([]systemd.AppConfig, 0, len(rawApps))}
	for _, rawApp := range rawApps {
		app, err := parseEcosystemApp(rawApp, envName)
		if err != nil {
			return systemd.EcosystemConfig{}, err
		}
		ecosystem.Apps = append(ecosystem.Apps, app)
	}

	return ecosystem, nil
}

func parseEcosystemApp(data []byte, envName string) (systemd.AppConfig, error) {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return systemd.AppConfig{}, fmt.Errorf("ecosystem app must be an object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return systemd.AppConfig{}, fmt.Errorf("invalid app config: %w", err)
	}
	if fields == nil {
		return systemd.AppConfig{}, fmt.Errorf("ecosystem app must be an object")
	}

	allowedFields := map[string]bool{
		"id":            true,
		"name":          true,
		"script":        true,
		"interpreter":   true,
		"cwd":           true,
		"args":          true,
		"env":           true,
		"out_file":      true,
		"error_file":    true,
		"log_file":      true,
		"autorestart":   true,
		"restart_delay": true,
	}

	for key := range fields {
		if allowedFields[key] || strings.HasPrefix(key, "env_") {
			continue
		}
		return systemd.AppConfig{}, fmt.Errorf("unsupported ecosystem app field %q", key)
	}
	for key, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return systemd.AppConfig{}, fmt.Errorf("ecosystem app field %q must not be null", key)
		}
	}

	var app systemd.AppConfig
	if err := json.Unmarshal(data, &app); err != nil {
		return systemd.AppConfig{}, fmt.Errorf("invalid app config: %w", err)
	}
	if strings.TrimSpace(app.Script) == "" {
		return systemd.AppConfig{}, fmt.Errorf("ecosystem app %q requires a non-empty script", app.Name)
	}
	for key, rawEnv := range fields {
		if !strings.HasPrefix(key, "env_") {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(rawEnv), []byte("null")) {
			return systemd.AppConfig{}, fmt.Errorf("invalid %s for app %q: expected an object", key, app.Name)
		}
		var profile map[string]string
		if err := json.Unmarshal(rawEnv, &profile); err != nil {
			return systemd.AppConfig{}, fmt.Errorf("invalid %s for app %q: %w", key, app.Name, err)
		}
	}

	if envName != "" {
		envField := "env_" + envName
		rawEnv, ok := fields[envField]
		if !ok {
			return app, nil
		}

		var selectedEnv map[string]string
		if err := json.Unmarshal(rawEnv, &selectedEnv); err != nil {
			return systemd.AppConfig{}, fmt.Errorf("invalid %s for app %q: %w", envField, app.Name, err)
		}
		app.Env = mergeEnvironment(app.Env, selectedEnv)
	}

	return app, nil
}

func parseStartEnvFlags(envVars []string) (string, map[string]string, error) {
	envOverrides := make(map[string]string)
	var envName string

	for _, envVar := range envVars {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			envOverrides[parts[0]] = parts[1]
			continue
		}

		if envName != "" && envName != envVar {
			return "", nil, fmt.Errorf("multiple ecosystem environments specified: %s, %s", envName, envVar)
		}
		envName = envVar
	}

	return envName, envOverrides, nil
}

func currentEnvironment() map[string]string {
	envMap := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	return envMap
}

func mergeEnvironment(base map[string]string, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(overrides))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}
	return merged
}
