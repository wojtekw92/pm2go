package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload [name|id|all]",
	Short: "Reload applications",
	Long: `Reload a specific application by name/id or reload all applications.

PM2go currently maps reload to systemd restart. This is not PM2's zero-downtime
cluster reload behavior yet.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		handleReload(args[0])
	},
}

func handleReload(identifier string) {
	fmt.Fprintln(os.Stderr, "Warning: reload currently uses restart semantics; zero-downtime reload is not implemented yet")
	handleRestart(identifier)
}
