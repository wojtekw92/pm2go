package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var resurrectCmd = &cobra.Command{
	Use:   "resurrect",
	Short: "Start applications in the saved systemd target",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := manager.Resurrect(); err != nil {
			fmt.Fprintf(os.Stderr, "Error resurrecting process list: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Started saved systemd applications")
	},
}
