package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop an application",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		handleStop(args[0])
	},
}

func handleStop(appName string) {
	if appName == "all" {
		handleStopAll()
		return
	}

	if err := manager.Stop(appName); err != nil {
		fmt.Printf("Error stopping %s: %v\n", appName, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Stopped %s\n", appName)
}

func handleStopAll() {
	processes, err := manager.List()
	if err != nil {
		fmt.Printf("Error getting process list: %v\n", err)
		os.Exit(1)
	}

	if len(processes) == 0 {
		fmt.Println("No processes to stop")
		return
	}

	successCount := 0
	errorCount := 0
	for _, process := range processes {
		identifier := strconv.Itoa(process.PM2Env.ID)
		if err := manager.Stop(identifier); err != nil {
			fmt.Printf("✗ Failed to stop %s (ID: %d): %v\n", process.Name, process.PM2Env.ID, err)
			errorCount++
		} else {
			fmt.Printf("✓ Stopped %s (ID: %d)\n", process.Name, process.PM2Env.ID)
			successCount++
		}
	}

	fmt.Printf("\nStop summary: %d successful, %d failed\n", successCount, errorCount)
	if errorCount > 0 {
		os.Exit(1)
	}
}
