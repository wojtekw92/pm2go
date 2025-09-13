package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/wojtekw92/pm2go/internal/table"
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

	// Create table with headers
	tbl := table.NewTable("id", "name", "pid", "status", "restart", "uptime", "↺", "memory", "cpu")

	// Add each process as a row
	for _, process := range processes {
		uptime := formatUptime(process.PM2Env.PMUptime)
		memory := formatMemory(process.Monit.Memory)
		
		tbl.AddRow(
			strconv.Itoa(process.PM2Env.ID),
			process.Name,
			strconv.Itoa(process.PID),
			process.PM2Env.Status,
			strconv.Itoa(process.PM2Env.RestartTime),
			uptime,
			strconv.Itoa(process.PM2Env.RestartTime),
			memory,
			fmt.Sprintf("%d%%", process.Monit.CPU),
		)
	}

	// Print the table
	tbl.Print()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatUptime converts milliseconds to human readable format
func formatUptime(uptimeMs int64) string {
	if uptimeMs == 0 {
		return "0s"
	}
	
	duration := time.Duration(uptimeMs) * time.Millisecond
	
	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60
	
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	} else if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

// formatMemory converts bytes to human readable format
func formatMemory(bytes int) string {
	if bytes == 0 {
		return "0b"
	}
	
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%db", bytes)
	}
	
	exp := 0
	for n := bytes; n >= unit && exp < 3; n /= unit {
		exp++
	}
	
	divisor := 1
	for i := 0; i < exp; i++ {
		divisor *= 1024
	}
	
	result := float64(bytes) / float64(divisor)
	units := []string{"b", "kb", "mb", "gb"}
	
	if result >= 100 {
		return fmt.Sprintf("%.0f%s", result, units[exp])
	} else if result >= 10 {
		return fmt.Sprintf("%.1f%s", result, units[exp])
	} else {
		return fmt.Sprintf("%.2f%s", result, units[exp])
	}
}