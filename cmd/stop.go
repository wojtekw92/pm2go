package cmd

import (
	"fmt"
	"os"

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
	if err := manager.Stop(appName); err != nil {
		fmt.Printf("Error stopping %s: %v\n", appName, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Stopped %s\n", appName)
}