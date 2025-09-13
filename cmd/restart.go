package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart [name|id|all]",
	Short: "Restart applications",
	Long: `Restart a specific application by name/id or restart all applications.

Examples:
  pm2go restart my-app       # Restart specific application by name
  pm2go restart 0            # Restart specific application by ID
  pm2go restart all          # Restart all applications`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Error: Please specify an application name, ID, or 'all'")
			os.Exit(1)
		}
		handleRestart(args[0])
	},
}

func handleRestart(identifier string) {
	if identifier == "all" {
		handleRestartAll()
		return
	}
	
	var appName string
	var targetID int
	
	// Try to resolve ID to app name
	if id, err := strconv.Atoi(identifier); err == nil {
		// It's a numeric ID
		processes, err := manager.List()
		if err != nil {
			fmt.Printf("Error getting process list: %v\n", err)
			os.Exit(1)
		}
		
		found := false
		for _, process := range processes {
			if process.PM2Env.ID == id {
				appName = process.Name
				targetID = id
				found = true
				break
			}
		}
		
		if !found {
			fmt.Printf("Error: Process with ID %d not found\n", id)
			os.Exit(1)
		}
	} else {
		// It's an app name - find the ID
		processes, err := manager.List()
		if err != nil {
			fmt.Printf("Error getting process list: %v\n", err)
			os.Exit(1)
		}
		
		found := false
		for _, process := range processes {
			if process.Name == identifier {
				appName = identifier
				targetID = process.PM2Env.ID
				found = true
				break
			}
		}
		
		if !found {
			fmt.Printf("Error: Process '%s' not found\n", identifier)
			os.Exit(1)
		}
	}
	
	// Restart the specific process
	if err := manager.Restart(targetID); err != nil {
		fmt.Printf("Error restarting %s: %v\n", appName, err)
		os.Exit(1)
	}
	
	fmt.Printf("✓ Restarted %s (ID: %d)\n", appName, targetID)
}

func handleRestartAll() {
	// Get all processes
	processes, err := manager.List()
	if err != nil {
		fmt.Printf("Error getting process list: %v\n", err)
		os.Exit(1)
	}
	
	if len(processes) == 0 {
		fmt.Println("No processes to restart")
		return
	}
	
	fmt.Printf("Restarting %d processes...\n", len(processes))
	
	successCount := 0
	errorCount := 0
	
	for _, process := range processes {
		if err := manager.Restart(process.PM2Env.ID); err != nil {
			fmt.Printf("✗ Failed to restart %s (ID: %d): %v\n", process.Name, process.PM2Env.ID, err)
			errorCount++
		} else {
			fmt.Printf("✓ Restarted %s (ID: %d)\n", process.Name, process.PM2Env.ID)
			successCount++
		}
	}
	
	fmt.Printf("\nRestart summary: %d successful, %d failed\n", successCount, errorCount)
	
	if errorCount > 0 {
		os.Exit(1)
	}
}