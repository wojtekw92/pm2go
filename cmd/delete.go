package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"del"},
	Short:   "Delete an application",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		handleDelete(args[0])
	},
}

func handleDelete(appName string) {
	if appName == "all" {
		handleDeleteAll()
		return
	}

	if err := manager.Delete(appName); err != nil {
		fmt.Printf("Error deleting %s: %v\n", appName, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Deleted %s\n", appName)
}

func handleDeleteAll() {
	processes, err := manager.List()
	if err != nil {
		fmt.Printf("Error getting process list: %v\n", err)
		os.Exit(1)
	}

	if len(processes) == 0 {
		fmt.Println("No processes to delete")
		return
	}

	successCount := 0
	errorCount := 0
	for _, process := range processes {
		identifier := strconv.Itoa(process.PM2Env.ID)
		if err := manager.Delete(identifier); err != nil {
			fmt.Printf("✗ Failed to delete %s (ID: %d): %v\n", process.Name, process.PM2Env.ID, err)
			errorCount++
		} else {
			fmt.Printf("✓ Deleted %s (ID: %d)\n", process.Name, process.PM2Env.ID)
			successCount++
		}
	}

	fmt.Printf("\nDelete summary: %d successful, %d failed\n", successCount, errorCount)
	if errorCount > 0 {
		os.Exit(1)
	}
}
