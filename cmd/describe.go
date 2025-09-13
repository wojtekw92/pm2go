package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/wojtekw92/pm2go/internal/table"
	"github.com/wojtekw92/pm2go/pkg/systemd"
)

var describeCmd = &cobra.Command{
	Use:     "describe <name|id>",
	Aliases: []string{"desc", "show"},
	Short:   "Describe process metadata",
	Long: `Show detailed information about a specific process.

Examples:
  pm2go describe my-app       # Describe process by name
  pm2go describe 0            # Describe process by ID`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		handleDescribe(args[0])
	},
}

func handleDescribe(identifier string) {
	var appName string
	var targetProcess *systemd.ProcessInfo
	
	// Get all processes
	processes, err := manager.List()
	if err != nil {
		fmt.Printf("Error getting process list: %v\n", err)
		os.Exit(1)
	}
	
	// Try to resolve ID to app name
	if id, err := strconv.Atoi(identifier); err == nil {
		// It's a numeric ID
		found := false
		for _, process := range processes {
			if process.PM2Env.ID == id {
				appName = process.Name
				targetProcess = &process
				found = true
				break
			}
		}
		
		if !found {
			fmt.Printf("Error: Process with ID %d not found\n", id)
			os.Exit(1)
		}
	} else {
		// It's an app name
		appName = identifier
		found := false
		for _, process := range processes {
			if process.Name == appName {
				targetProcess = &process
				found = true
				break
			}
		}
		
		if !found {
			fmt.Printf("Error: Process '%s' not found\n", appName)
			os.Exit(1)
		}
	}
	
	// Display detailed information
	fmt.Printf("Describing process with id %d - name %s\n", targetProcess.PM2Env.ID, targetProcess.Name)
	
	// Create main info table
	mainTable := table.NewKeyValueTable().
		SetKeyWidth(17).
		SetValueWidth(88)
	
	mainTable.
		AddKeyValue("status", targetProcess.PM2Env.Status).
		AddKeyValue("name", targetProcess.Name).
		AddKeyValue("namespace", "default").
		AddKeyValue("version", "N/A").
		AddKeyValue("restarts", strconv.Itoa(targetProcess.PM2Env.RestartTime)).
		AddKeyValue("uptime", formatUptime(targetProcess.PM2Env.PMUptime)).
		AddKeyValue("script path", targetProcess.PM2Env.Interpreter).
		AddKeyValue("script args", getScriptWithArgs(targetProcess)).
		AddKeyValue("error log path", targetProcess.PM2Env.PMErrLogPath).
		AddKeyValue("out log path", targetProcess.PM2Env.PMOutLogPath).
		AddKeyValue("pid path", targetProcess.PM2Env.PMPidPath).
		AddKeyValue("interpreter", getInterpreterName(targetProcess)).
		AddKeyValue("interpreter args", "N/A").
		AddKeyValue("script id", strconv.Itoa(targetProcess.PM2Env.ID)).
		AddKeyValue("exec cwd", getCurrentWorkingDir()).
		AddKeyValue("exec mode", "fork_mode").
		AddKeyValue("node.js version", "N/A").
		AddKeyValue("node env", "N/A").
		AddKeyValue("watch & reload", "✘").
		AddKeyValue("unstable restarts", strconv.Itoa(targetProcess.PM2Env.UnstableRestarts)).
		AddKeyValue("created at", formatTimestamp(targetProcess.PM2Env.CreatedAt))
	
	mainTable.Print()
	
	// Show divergent environment variables
	showDivergentEnvVars(targetProcess.PM2Env.Env)
}

func getInterpreter(process *systemd.ProcessInfo) string {
	if process.PM2Env.Interpreter != "" {
		return process.PM2Env.Interpreter
	}
	return "node"
}

func getArgs(process *systemd.ProcessInfo) string {
	if process.PM2Env.Args != "" {
		return process.PM2Env.Args
	}
	return "N/A"
}

func formatTimestamp(timestamp int64) string {
	if timestamp == 0 {
		return "N/A"
	}
	// Convert from milliseconds to time and format
	t := time.Unix(timestamp/1000, 0)
	return t.Format("2006-01-02T15:04:05.000Z")
}


func getScriptWithArgs(process *systemd.ProcessInfo) string {
	script := process.PM2Env.PMExecPath
	args := getArgs(process)
	if args != "N/A" && args != "" {
		return script + " " + args
	}
	return script
}

func getInterpreterName(process *systemd.ProcessInfo) string {
	if process.PM2Env.Interpreter == "" {
		return "none"
	}
	// Extract just the interpreter name from the full path
	interpreter := process.PM2Env.Interpreter
	if lastSlash := strings.LastIndex(interpreter, "/"); lastSlash != -1 {
		return interpreter[lastSlash+1:]
	}
	return interpreter
}

func getCurrentWorkingDir() string {
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "N/A"
}

func showDivergentEnvVars(processEnv map[string]string) {
	// Get current environment
	currentEnv := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			currentEnv[parts[0]] = parts[1]
		}
	}
	
	// Collect divergent variables first to calculate optimal widths
	var divergentVars [][]string
	for key, processValue := range processEnv {
		currentValue, exists := currentEnv[key]
		if !exists || currentValue != processValue {
			divergentVars = append(divergentVars, []string{key, processValue})
		}
	}
	
	if len(divergentVars) > 0 {
		// Calculate optimal column widths based on actual divergent variables
		maxKeyWidth := utf8.RuneCountInString("Key") // Start with header width
		maxValueWidth := utf8.RuneCountInString("Value") // Start with header width
		
		for _, pair := range divergentVars {
			key := pair[0]
			value := pair[1]
			
			if utf8.RuneCountInString(key) > maxKeyWidth {
				maxKeyWidth = utf8.RuneCountInString(key)
			}
			if utf8.RuneCountInString(value) > maxValueWidth {
				maxValueWidth = utf8.RuneCountInString(value)
			}
		}
		
		// Create table with dynamic widths based on content
		divergentTable := table.NewKeyValueTable().
			SetKeyWidth(maxKeyWidth).
			SetValueWidth(maxValueWidth)
		
		// Add all divergent variables
		for _, pair := range divergentVars {
			divergentTable.AddKeyValue(pair[0], pair[1])
		}
		
		fmt.Println("Divergent env variables from local env")
		divergentTable.Print()
	}
}