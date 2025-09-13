package cmd

import (
	"fmt"
	"os"
	"strconv"
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
	fmt.Println("┌─────────────────────────────────────────────────────────────┐")
	fmt.Printf("│ status             │ %-39s │\n", targetProcess.PM2Env.Status)
	fmt.Printf("│ name               │ %-39s │\n", targetProcess.Name)
	fmt.Printf("│ id                 │ %-39d │\n", targetProcess.PM2Env.ID)
	fmt.Printf("│ pid                │ %-39d │\n", targetProcess.PID)
	fmt.Printf("│ interpreter        │ %-39s │\n", getInterpreter(targetProcess))
	fmt.Printf("│ script             │ %-39s │\n", targetProcess.PM2Env.PMExecPath)
	fmt.Printf("│ args               │ %-39s │\n", getArgs(targetProcess))
	fmt.Printf("│ restart time       │ %-39d │\n", targetProcess.PM2Env.RestartTime)
	fmt.Printf("│ uptime             │ %-39s │\n", formatUptime(targetProcess.PM2Env.PMUptime))
	fmt.Printf("│ memory usage       │ %-39s │\n", formatMemory(targetProcess.Monit.Memory))
	fmt.Printf("│ cpu usage          │ %-39d%% │\n", targetProcess.Monit.CPU)
	fmt.Println("├─────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ script path        │ %-39s │\n", targetProcess.PM2Env.PMExecPath)
	fmt.Printf("│ script args        │ %-39s │\n", getArgs(targetProcess))
	fmt.Printf("│ error log path     │ %-39s │\n", targetProcess.PM2Env.PMErrLogPath)
	fmt.Printf("│ out log path       │ %-39s │\n", targetProcess.PM2Env.PMOutLogPath)
	fmt.Printf("│ pid path           │ %-39s │\n", targetProcess.PM2Env.PMPidPath)
	fmt.Println("├─────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ exec interpreter   │ %-39s │\n", getInterpreter(targetProcess))
	fmt.Printf("│ exec mode          │ %-39s │\n", "fork_mode")
	fmt.Printf("│ node.js version    │ %-39s │\n", "N/A")
	fmt.Printf("│ watch & reload     │ %-39s │\n", "✘")
	fmt.Printf("│ unstable restarts  │ %-39d │\n", 0)
	fmt.Printf("│ created at         │ %-39s │\n", formatTimestamp(targetProcess.PM2Env.CreatedAt))
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	
	// Environment variables section
	if len(targetProcess.PM2Env.Env) > 0 {
		fmt.Println("\nEnvironment:")
		for key, value := range targetProcess.PM2Env.Env {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}
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
	return t.Format("2006-01-02 15:04:05")
}