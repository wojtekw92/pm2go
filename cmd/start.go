package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wojtekw92/pm2go/pkg/systemd"
)

var startCmd = &cobra.Command{
	Use:                "start [interpreter] -- [script] [args...] | start [script|ecosystem.json]",
	Short:              "Start an application or ecosystem",
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		envVars, _ := cmd.Flags().GetStringSlice("env")

		// Parse os.Args to properly handle "--" separator that Cobra consumes
		rawArgs := parseRawArgs()
		handleStart(rawArgs, name, envVars)
	},
}

func init() {
	startCmd.Flags().StringP("name", "n", "", "Application name")
	startCmd.Flags().StringSliceP("env", "e", []string{}, "Environment variables (KEY=VALUE)")
}

// parseRawArgs extracts the arguments for "start" command from os.Args, preserving "--"
func parseRawArgs() []string {
	args := os.Args
	startIndex := -1

	// Find "start" command
	for i, arg := range args {
		if arg == "start" {
			startIndex = i
			break
		}
	}

	if startIndex == -1 || startIndex >= len(args)-1 {
		return []string{}
	}

	// Extract arguments after "start", skipping flags
	var result []string
	for i := startIndex + 1; i < len(args); i++ {
		arg := args[i]
		// Skip flags and their values
		if strings.HasPrefix(arg, "--name") || strings.HasPrefix(arg, "-n") {
			if strings.Contains(arg, "=") {
				// --name=value format, skip just this arg
				continue
			} else {
				// --name value format, skip this arg and next
				if i+1 < len(args) {
					i++ // skip the value
				}
				continue
			}
		}
		if strings.HasPrefix(arg, "--env") || strings.HasPrefix(arg, "-e") {
			if strings.Contains(arg, "=") {
				// --env=value format, skip just this arg
				continue
			} else {
				// --env value format, skip this arg and next
				if i+1 < len(args) {
					i++ // skip the value
				}
				continue
			}
		}
		result = append(result, arg)
	}

	return result
}

func handleStart(args []string, name string, envVars []string) {
	// Check if we're restarting an existing process by ID
	if len(args) == 1 {
		if id, err := strconv.Atoi(args[0]); err == nil {
			// It's a numeric ID - restart existing process
			handleStartRestart(id)
			return
		}

		// Check if it's an ecosystem file
		if strings.HasSuffix(args[0], ".json") || strings.Contains(args[0], "ecosystem") {
			handleEcosystemStart(args[0], envVars)
			return
		}
	}

	var config systemd.AppConfig
	config.Env = make(map[string]string)

	// Parse arguments: either "script" or "interpreter -- script args..."
	if len(args) == 1 {
		// Simple case: pm2go start script.py
		config.Script = args[0]
	} else {
		// Complex case: pm2go start python3 -- script.py -arg1 -arg2
		// Find the "--" separator
		separatorIndex := -1
		for i, arg := range args {
			if arg == "--" {
				separatorIndex = i
				break
			}
		}

		if separatorIndex == -1 {
			// No separator found, treat first arg as script, rest as arguments
			config.Script = args[0]
			if len(args) > 1 {
				config.Args = strings.Join(args[1:], " ")
			}
		} else {
			// Separator found: interpreter -- script args...
			if separatorIndex == 0 {
				fmt.Println("Error: No interpreter specified before '--'")
				os.Exit(1)
			}
			if separatorIndex >= len(args)-1 {
				fmt.Println("Error: No script specified after '--'")
				os.Exit(1)
			}

			config.Interpreter = strings.Join(args[:separatorIndex], " ")
			config.Script = args[separatorIndex+1]
			if len(args) > separatorIndex+2 {
				config.Args = strings.Join(args[separatorIndex+2:], " ")
			}
		}
	}

	// Add all environment variables (inherit from parent process)
	config.Env = currentEnvironment()

	// Add/override with command-line environment variables
	for _, envVar := range envVars {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			config.Env[parts[0]] = parts[1]
		} else {
			fmt.Printf("Warning: Invalid environment variable format: %s (expected KEY=VALUE)\n", envVar)
		}
	}

	if name == "" {
		// Generate name from script filename
		name = strings.TrimSuffix(filepath.Base(config.Script), filepath.Ext(config.Script))
	}

	config.Name = name

	if err := manager.Start(config); err != nil {
		fmt.Printf("Error starting %s: %v\n", name, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Started %s\n", name)
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

	for _, app := range ecosystem.Apps {
		app.Env = mergeEnvironment(currentEnvironment(), app.Env)
		app.Env = mergeEnvironment(app.Env, envOverrides)
		if app.Name == "" {
			app.Name = strings.TrimSuffix(filepath.Base(app.Script), filepath.Ext(app.Script))
		}
		if err := manager.Start(app); err != nil {
			fmt.Printf("Error starting %s: %v\n", app.Name, err)
		} else {
			fmt.Printf("✓ Started %s\n", app.Name)
		}
	}
}

func parseEcosystemConfig(data []byte, envName string) (systemd.EcosystemConfig, error) {
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return systemd.EcosystemConfig{}, fmt.Errorf("unsupported or invalid ecosystem option: %w", err)
	}

	for key := range raw {
		if key != "apps" {
			return systemd.EcosystemConfig{}, fmt.Errorf("unsupported ecosystem field %q", key)
		}
	}

	var rawApps []json.RawMessage
	if err := json.Unmarshal(raw["apps"], &rawApps); err != nil {
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
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return systemd.AppConfig{}, fmt.Errorf("invalid app config: %w", err)
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
		"merge_logs":    true,
		"combine_logs":  true,
		"autorestart":   true,
		"restart_delay": true,
		"max_restarts":  true,
	}

	for key := range fields {
		if allowedFields[key] || strings.HasPrefix(key, "env_") {
			continue
		}
		return systemd.AppConfig{}, fmt.Errorf("unsupported ecosystem app field %q", key)
	}

	var app systemd.AppConfig
	if err := json.Unmarshal(data, &app); err != nil {
		return systemd.AppConfig{}, fmt.Errorf("invalid app config: %w", err)
	}

	if envName != "" {
		envField := "env_" + envName
		rawEnv, ok := fields[envField]
		if !ok {
			return systemd.AppConfig{}, fmt.Errorf("requested --env %s but %s is not defined for app %q", envName, envField, app.Name)
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
