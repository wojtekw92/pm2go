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
	Use:                "start [interpreter] -- [script] [args...] | start [script|ecosystem.json]",
	Short:              "Start an application or ecosystem",
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		envVars, _ := cmd.Flags().GetStringSlice("env")
		
		// Parse os.Args to properly handle "--" separator that Cobra consumes
		rawArgs := parseRawArgs()
		fmt.Printf("DEBUG: Raw args from os.Args: %v\n", rawArgs)
		
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
	fmt.Printf("DEBUG: Raw args: %v\n", args)
	
	// Check if it's an ecosystem file (single argument)
	if len(args) == 1 && (strings.HasSuffix(args[0], ".json") || strings.Contains(args[0], "ecosystem")) {
		handleEcosystemStart(args[0])
		return
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
			fmt.Printf("DEBUG: Parsed - Interpreter: '%s', Script: '%s', Args: '%s'\n", config.Interpreter, config.Script, config.Args)
		}
	}

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