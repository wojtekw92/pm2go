package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wojtekw92/pm2go/pkg/systemd"
)

var describeCmd = &cobra.Command{
	Use:     "describe <name|id>",
	Aliases: []string{"desc", "show"},
	Short:   "Describe process metadata",
	Long: `Show detailed information about a specific process.

Examples:
  pm2go describe my-app       # Describe process by name
  pm2go describe 0            # Describe process by ID`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		handleDescribe(args[0])
	},
}

func handleDescribe(identifier string) {
	var appName string
	var targetProcess *systemd.ProcessInfo
	
	// Get all processes
	processes, err := manager.List()
	if err != nil {
		fmt.Printf("Error getting process list: %v\n", err)
		os.Exit(1)
	}
	
	// Try to resolve ID to app name
	if id, err := strconv.Atoi(identifier); err == nil {
		// It's a numeric ID
		found := false
		for _, process := range processes {
			if process.PM2Env.ID == id {
				appName = process.Name
				targetProcess = &process
				found = true
				break
			}
		}
		
		if !found {
			fmt.Printf("Error: Process with ID %d not found\n", id)
			os.Exit(1)
		}
	} else {
		// It's an app name
		appName = identifier
		found := false
		for _, process := range processes {
			if process.Name == appName {
				targetProcess = &process
				found = true
				break
			}
		}
		
		if !found {
			fmt.Printf("Error: Process '%s' not found\n", appName)
			os.Exit(1)
		}
	}
	
	// Display detailed information
	fmt.Printf("Describing process with id %d - name %s\n", targetProcess.PM2Env.ID, targetProcess.Name)
	fmt.Println("┌───────────────────┬──────────────────────────────────────────────────────────────────────────────────────────┐")
	fmt.Printf("│ %-17s │ %-88s │\n", "status", targetProcess.PM2Env.Status)
	fmt.Printf("│ %-17s │ %-88s │\n", "name", targetProcess.Name)
	fmt.Printf("│ %-17s │ %-88s │\n", "namespace", "default")
	fmt.Printf("│ %-17s │ %-88s │\n", "version", "N/A")
	fmt.Printf("│ %-17s │ %-88d │\n", "restarts", targetProcess.PM2Env.RestartTime)
	fmt.Printf("│ %-17s │ %-88s │\n", "uptime", formatUptime(targetProcess.PM2Env.PMUptime))
	fmt.Printf("│ %-17s │ %-88s │\n", "script path", truncateField(targetProcess.PM2Env.Interpreter, 88))
	fmt.Printf("│ %-17s │ %-88s │\n", "script args", truncateField(getScriptWithArgs(targetProcess), 88))
	fmt.Printf("│ %-17s │ %-88s │\n", "error log path", truncateField(targetProcess.PM2Env.PMErrLogPath, 88))
	fmt.Printf("│ %-17s │ %-88s │\n", "out log path", truncateField(targetProcess.PM2Env.PMOutLogPath, 88))
	fmt.Printf("│ %-17s │ %-88s │\n", "pid path", truncateField(targetProcess.PM2Env.PMPidPath, 88))
	fmt.Printf("│ %-17s │ %-88s │\n", "interpreter", getInterpreterName(targetProcess))
	fmt.Printf("│ %-17s │ %-88s │\n", "interpreter args", "N/A")
	fmt.Printf("│ %-17s │ %-88d │\n", "script id", targetProcess.PM2Env.ID)
	fmt.Printf("│ %-17s │ %-88s │\n", "exec cwd", truncateField(getCurrentWorkingDir(), 88))
	fmt.Printf("│ %-17s │ %-88s │\n", "exec mode", "fork_mode")
	fmt.Printf("│ %-17s │ %-88s │\n", "node.js version", "N/A")
	fmt.Printf("│ %-17s │ %-88s │\n", "node env", "N/A")
	fmt.Printf("│ %-17s │ %-88s │\n", "watch & reload", "✘")
	fmt.Printf("│ %-17s │ %-88d │\n", "unstable restarts", targetProcess.PM2Env.UnstableRestarts)
	fmt.Printf("│ %-17s │ %-88s │\n", "created at", formatTimestamp(targetProcess.PM2Env.CreatedAt))
	fmt.Println("└───────────────────┴──────────────────────────────────────────────────────────────────────────────────────────┘")
	
	// Show divergent environment variables
	showDivergentEnvVars(targetProcess.PM2Env.Env)
}

func getInterpreter(process *systemd.ProcessInfo) string {
	if process.PM2Env.Interpreter != "" {
		return process.PM2Env.Interpreter
	}
	return "node"
}

func getArgs(process *systemd.ProcessInfo) string {
	if process.PM2Env.Args != "" {
		return process.PM2Env.Args
	}
	return "N/A"
}

func formatTimestamp(timestamp int64) string {
	if timestamp == 0 {
		return "N/A"
	}
	// Convert from milliseconds to time and format
	t := time.Unix(timestamp/1000, 0)
	return t.Format("2006-01-02T15:04:05.000Z")
}

func truncateField(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func getScriptWithArgs(process *systemd.ProcessInfo) string {
	script := process.PM2Env.PMExecPath
	args := getArgs(process)
	if args != "N/A" && args != "" {
		return script + " " + args
	}
	return script
}

func getInterpreterName(process *systemd.ProcessInfo) string {
	if process.PM2Env.Interpreter == "" {
		return "none"
	}
	// Extract just the interpreter name from the full path
	interpreter := process.PM2Env.Interpreter
	if lastSlash := strings.LastIndex(interpreter, "/"); lastSlash != -1 {
		return interpreter[lastSlash+1:]
	}
	return interpreter
}

func getCurrentWorkingDir() string {
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "N/A"
}

func showDivergentEnvVars(processEnv map[string]string) {
	// Get current environment
	currentEnv := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			currentEnv[parts[0]] = parts[1]
		}
	}
	
	// Find divergent variables
	var divergent [][]string
	for key, processValue := range processEnv {
		currentValue, exists := currentEnv[key]
		if !exists || currentValue != processValue {
			divergent = append(divergent, []string{key, processValue})
		}
	}
	
	if len(divergent) > 0 {
		fmt.Println("Divergent env variables from local env")
		fmt.Println("┌──────────────────┬─────────────────────────────────────────────────────────────────────────────────────────────────┐")
		
		for _, pair := range divergent {
			key := pair[0]
			value := pair[1]
			fmt.Printf("│ %-16s │ %-99s │\n", truncateField(key, 16), truncateField(value, 99))
		}
		
		fmt.Println("└──────────────────┴─────────────────────────────────────────────────────────────────────────────────────────────────┘")
	}
}