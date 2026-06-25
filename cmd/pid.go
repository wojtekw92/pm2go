package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var pidCmd = &cobra.Command{
	Use:   "pid <name|id>",
	Short: "Print process PID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		handlePid(args[0])
	},
}

func handlePid(identifier string) {
	processes, err := manager.List()
	if err != nil {
		fmt.Printf("Error getting process list: %v\n", err)
		os.Exit(1)
	}

	for _, process := range processes {
		if process.Name == identifier || strconv.Itoa(process.PM2Env.ID) == identifier {
			fmt.Println(process.PID)
			return
		}
	}

	fmt.Printf("Error: Process '%s' not found\n", identifier)
	os.Exit(1)
}
