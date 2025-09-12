package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wojtekw92/pm2go/pkg/systemd"
)

var manager *systemd.Manager

var rootCmd = &cobra.Command{
	Use:   "pm2go",
	Short: "PM2 Systemd Wrapper",
	Long:  `A PM2 reimplementation using systemd for process management.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	manager = systemd.NewManager()

	// Add all commands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(flushCmd)
	rootCmd.AddCommand(jlistCmd)
	rootCmd.AddCommand(startupCmd)
	rootCmd.AddCommand(logsCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}