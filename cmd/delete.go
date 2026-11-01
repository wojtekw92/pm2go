package cmd

import (
	"fmt"
	"os"

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

	if err := manager.Delete("all"); err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting all processes: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Deleted %d processes\n", len(processes))
}
