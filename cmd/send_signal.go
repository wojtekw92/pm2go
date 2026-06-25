package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var sendSignalCmd = &cobra.Command{
	Use:   "sendSignal <signal> <name|id>",
	Short: "Send signal to a process",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		handleSendSignal(args[0], args[1])
	},
}

func handleSendSignal(signal string, identifier string) {
	signal = normalizeSignal(signal)
	if err := manager.SendSignal(identifier, signal); err != nil {
		fmt.Printf("Error sending %s to %s: %v\n", signal, identifier, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Sent %s to %s\n", signal, identifier)
}

func normalizeSignal(signal string) string {
	signal = strings.TrimSpace(strings.ToUpper(signal))
	if signal == "" || strings.HasPrefix(signal, "SIG") {
		return signal
	}
	return "SIG" + signal
}
