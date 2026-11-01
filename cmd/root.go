package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wojtekw92/pm2go/pkg/systemd"
)

var manager *systemd.Manager

// version is set at build time via -ldflags "-X github.com/wojtekw92/pm2go/cmd.version=<tag>".
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "pm2go",
	Short: "PM2 usage compatibility backed by systemd",
	Long:  `A compatibility layer that translates supported PM2 workflows into native systemd operations.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	manager = systemd.NewManager()
	rootCmd.Version = version

	// Add all commands
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(flushCmd)
	rootCmd.AddCommand(jlistCmd)
	rootCmd.AddCommand(startupCmd)
	rootCmd.AddCommand(saveCmd)
	rootCmd.AddCommand(resurrectCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(describeCmd)
	rootCmd.AddCommand(envCmd)
	rootCmd.AddCommand(pidCmd)
	rootCmd.AddCommand(reloadCmd)
	rootCmd.AddCommand(sendSignalCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(hubCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
