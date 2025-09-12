package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var jlistCmd = &cobra.Command{
	Use:   "jlist",
	Short: "List all applications in JSON format (PM2 compatible)",
	Run: func(cmd *cobra.Command, args []string) {
		handleJList()
	},
}

func handleJList() {
	processes, err := manager.List()
	if err != nil {
		fmt.Printf("Error listing processes: %v\n", err)
		os.Exit(1)
	}
	jsonOutput, err := json.MarshalIndent(processes, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonOutput))
}