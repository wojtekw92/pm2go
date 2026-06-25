package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/wojtekw92/pm2go/pkg/hub"
)

var hubAddr string
var hubSecret string
var hubDBPath string
var hubMemory bool
var hubDashboard bool

var hubCmd = &cobra.Command{
	Use:   "hub start",
	Short: "Run central PM2go control hub",
	Long: `Run the central PM2go control hub.

Agents running on process hosts connect outbound to this hub, publish process
inventory, fetch queued commands, execute them locally, and report results.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if args[0] != "start" {
			fmt.Printf("Unknown hub action: %s\n", args[0])
			os.Exit(1)
		}
		handleHubStart()
	},
}

func init() {
	hubCmd.Flags().StringVar(&hubAddr, "addr", ":9616", "HTTP listen address")
	hubCmd.Flags().StringVar(&hubSecret, "secret", "", "Bearer token for agents/operators (or PM2GO_HUB_SECRET)")
	hubCmd.Flags().StringVar(&hubDBPath, "db", "", "SQLite database path")
	hubCmd.Flags().BoolVar(&hubMemory, "memory", false, "Use in-memory hub storage")
	hubCmd.Flags().BoolVar(&hubDashboard, "dashboard", false, "Serve the hub dashboard at /dashboard")
}

func handleHubStart() {
	if hubSecret == "" {
		hubSecret = os.Getenv("PM2GO_HUB_SECRET")
	}
	if hubDBPath == "" {
		hubDBPath = defaultHubDBPath()
	}

	store := hub.Store(hub.NewMemoryStore())
	if !hubMemory {
		if err := os.MkdirAll(filepath.Dir(hubDBPath), 0755); err != nil {
			fmt.Printf("Error creating database directory: %v\n", err)
			os.Exit(1)
		}
		sqliteStore, err := hub.OpenSQLiteStore(hubDBPath)
		if err != nil {
			fmt.Printf("Error opening SQLite database: %v\n", err)
			os.Exit(1)
		}
		defer sqliteStore.Close()
		store = sqliteStore
	}

	hubServer := hub.NewServerWithStore(hubSecret, store)
	if hubDashboard {
		hubServer.EnableDashboard()
	}

	server := &http.Server{
		Addr:              hubAddr,
		Handler:           hubServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-done
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	fmt.Printf("PM2go hub listening on http://%s\n", hubAddr)
	if hubMemory {
		fmt.Println("Storage: memory")
	} else {
		fmt.Printf("Storage: sqlite %s\n", hubDBPath)
	}
	if hubSecret == "" {
		fmt.Println("Warning: hub authentication is disabled; set --secret or PM2GO_HUB_SECRET for production")
	}
	if hubDashboard {
		fmt.Printf("Dashboard: http://%s/dashboard\n", hubAddr)
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Printf("Error starting hub: %v\n", err)
		os.Exit(1)
	}
}

func defaultHubDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pm2", "pm2go-hub.db")
}
