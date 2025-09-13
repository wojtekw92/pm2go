package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/wojtekw92/pm2go/pkg/systemd"
)

var envCmd = &cobra.Command{
	Use:   "env <name|id>",
	Short: "Show environment variables for a process",
	Long: `Display all environment variables for a specific process.

Examples:
  pm2go env my-app       # Show environment variables for process by name
  pm2go env 0            # Show environment variables for process by ID`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		handleEnv(args[0])
	},
}

func handleEnv(identifier string) {
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
		found := false
		for _, process := range processes {
			if process.Name == identifier {
				targetProcess = &process
				found = true
				break
			}
		}
		
		if !found {
			fmt.Printf("Error: Process '%s' not found\n", identifier)
			os.Exit(1)
		}
	}
	
	// Display environment variables
	if targetProcess.PM2Env.Env == nil || len(targetProcess.PM2Env.Env) == 0 {
		fmt.Println("No environment variables found for this process")
		return
	}
	
	// Sort keys for consistent output
	keys := make([]string, 0, len(targetProcess.PM2Env.Env))
	for key := range targetProcess.PM2Env.Env {
		keys = append(keys, key)
	}
	
	// Simple alphabetical sort (basic implementation)
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	
	// Display each environment variable
	for _, key := range keys {
		value := targetProcess.PM2Env.Env[key]
		fmt.Printf("%s: %s\n", key, value)
	}
}