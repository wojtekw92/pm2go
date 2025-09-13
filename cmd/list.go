package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List all applications",
	Run: func(cmd *cobra.Command, args []string) {
		handleList()
	},
}

func handleList() {
	processes, err := manager.List()
	if err != nil {
		fmt.Printf("Error listing processes: %v\n", err)
		os.Exit(1)
	}

	// Print PM2-style table
	fmt.Println("┌─────┬──────────────────┬─────────────┬─────────┬─────────┬──────────┐")
	fmt.Println("│ id  │ name             │ mode        │ ↺      │ status  │ cpu      │")
	fmt.Println("├─────┼──────────────────┼─────────────┼─────────┼─────────┼──────────┤")

	for id, process := range processes {
		fmt.Printf("│ %-3d │ %-16s │ %-11s │ %-7d │ %-7s │ %-8s │\n",
			id, 
			truncateString(process.Name, 16), 
			process.PM2Env.ExecMode, 
			process.PM2Env.RestartTime, 
			process.PM2Env.Status, 
			fmt.Sprintf("%d%%", process.Monit.CPU))
	}
	fmt.Println("└─────┴──────────────────┴─────────────┴─────────┴─────────┴──────────┘")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}