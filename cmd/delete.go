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
	if err := manager.Delete(appName); err != nil {
		fmt.Printf("Error deleting %s: %v\n", appName, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Deleted %s\n", appName)
}