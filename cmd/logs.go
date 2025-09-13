package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [name]",
	Short: "Show logs for applications",
	Long: `Show logs for a specific application or all PM2go applications.
Uses systemd's journald for log management.

Examples:
  pm2go logs              # Show logs for all applications
  pm2go logs my-app       # Show logs for specific application
  pm2go logs my-app -f    # Follow logs in real-time
  pm2go logs -l 100       # Show last 100 lines`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var appName string
		if len(args) > 0 {
			appName = args[0]
		}
		
		lines, _ := cmd.Flags().GetInt("lines")
		follow, _ := cmd.Flags().GetBool("follow")
		
		handleLogs(appName, lines, follow)
	},
}

func init() {
	logsCmd.Flags().IntP("lines", "l", 50, "Number of lines to display")
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output (like tail -f)")
}

func handleLogs(identifier string, lines int, follow bool) {
	var appName string
	
	if identifier != "" {
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
		}
	}
	
	if err := manager.Logs(appName, lines, follow); err != nil {
		fmt.Printf("Error showing logs: %v\n", err)
		os.Exit(1)
	}
}