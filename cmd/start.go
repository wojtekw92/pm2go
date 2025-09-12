package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wojtekw92/pm2go/pkg/systemd"
)

var startCmd = &cobra.Command{
	Use:   "start [script|ecosystem.json]",
	Short: "Start an application or ecosystem",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		envVars, _ := cmd.Flags().GetStringSlice("env")
		handleStart(args[0], name, envVars)
	},
}

func init() {
	startCmd.Flags().StringP("name", "n", "", "Application name")
	startCmd.Flags().StringSliceP("env", "e", []string{}, "Environment variables (KEY=VALUE)")
}

func handleStart(scriptOrFile, name string, envVars []string) {
	// Check if it's an ecosystem file
	if strings.HasSuffix(scriptOrFile, ".json") || strings.Contains(scriptOrFile, "ecosystem") {
		handleEcosystemStart(scriptOrFile)
		return
	}

	var config systemd.AppConfig
	config.Script = scriptOrFile
	config.Env = make(map[string]string)

	// Add safe shell environment variables (inherit from parent process)
	safeEnvVars := []string{"PATH", "HOME", "USER", "NODE_ENV", "PYTHON_ENV", "PORT"}
	for _, envName := range safeEnvVars {
		if value := os.Getenv(envName); value != "" {
			config.Env[envName] = value
		}
	}

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

func handleEcosystemStart(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading ecosystem file: %v\n", err)
		os.Exit(1)
	}

	var ecosystem systemd.EcosystemConfig
	if err := json.Unmarshal(data, &ecosystem); err != nil {
		fmt.Printf("Error parsing ecosystem file: %v\n", err)
		os.Exit(1)
	}

	for _, app := range ecosystem.Apps {
		if err := manager.Start(app); err != nil {
			fmt.Printf("Error starting %s: %v\n", app.Name, err)
		} else {
			fmt.Printf("✓ Started %s\n", app.Name)
		}
	}
}