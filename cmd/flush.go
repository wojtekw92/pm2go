package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var flushCmd = &cobra.Command{
	Use:   "flush [name]",
	Short: "Remove logs (all logs or specific app logs)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var appName string
		if len(args) > 0 {
			appName = args[0]
		}
		handleFlush(appName)
	},
}

func handleFlush(appName string) {
	if err := manager.Flush(appName); err != nil {
		fmt.Printf("Error flushing logs: %v\n", err)
		os.Exit(1)
	}
	if appName == "" {
		fmt.Println("✓ Flushed all logs")
	} else {
		fmt.Printf("✓ Flushed logs for %s\n", appName)
	}
}