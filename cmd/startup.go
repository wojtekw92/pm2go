package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var startupCmd = &cobra.Command{
	Use:   "startup",
	Short: "Configure systemd for automatic startup at boot",
	Run: func(cmd *cobra.Command, args []string) {
		handleStartup()
	},
}

func handleStartup() {
	// Check if already configured
	if manager.IsStartupConfigured() {
		fmt.Println("✓ Startup is already configured!")
		lingering, userService := manager.GetStartupStatus()
		fmt.Printf("  Lingering enabled: %s\n", lingering)
		fmt.Printf("  User service enabled: %s\n", userService)
		fmt.Println()
		fmt.Println("Your PM2go services will start automatically at boot.")
		return
	}

	// Show what will be done
	username := os.Getenv("USER")
	uid := os.Getuid()

	fmt.Println("PM2go Systemd Wrapper - Configuring startup...")
	fmt.Println()
	fmt.Printf("Configuring systemd for user: %s (uid: %d)\n", username, uid)
	fmt.Println("This will allow your PM2go services to:")
	fmt.Println("  • Start automatically at boot")
	fmt.Println("  • Run without active login sessions")
	fmt.Println("  • Persist across system reboots")
	fmt.Println()
	fmt.Println("The following commands will be executed:")
	commands := manager.GetStartupCommands()
	for _, cmd := range commands {
		fmt.Printf("  %s\n", cmd)
	}
	fmt.Println()

	// Configure startup
	fmt.Print("Enabling user lingering... ")
	fmt.Print("Enabling user systemd service... ")
	fmt.Print("Reloading systemd configuration... ")

	if err := manager.ConfigureStartup(); err != nil {
		fmt.Printf("\n✗ Some configuration steps failed: %v\n\n", err)
		fmt.Println("You may need to run these commands manually:")
		for _, cmd := range commands {
			fmt.Printf("  %s\n", cmd)
		}
		os.Exit(1)
	}

	fmt.Println("✓")
	fmt.Println()
	fmt.Println("✓ Startup configuration completed successfully!")
	fmt.Println()
	fmt.Println("Your PM2go services will now:")
	fmt.Println("  • Start automatically when the system boots")
	fmt.Println("  • Continue running after you log out")
	fmt.Println("  • Restart automatically if they crash")
	fmt.Println()
	fmt.Println("You can now use:")
	fmt.Println("  pm2go start <app>     - to start applications")
	fmt.Println("  pm2go list           - to see running services")
	fmt.Println("  pm2go flush           - to clear application logs")
}