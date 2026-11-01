package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save online applications in the systemd startup target",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := manager.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving process list: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Saved online applications in systemd")
	},
}
