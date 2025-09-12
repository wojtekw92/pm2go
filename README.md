# PM2go - PM2 Reimplementation with systemd

A drop-in replacement for PM2 that uses systemd for robust process management on Linux systems. PM2go provides the same familiar PM2 interface while leveraging systemd's proven reliability for service management.

## Features

- 🔄 **Drop-in PM2 replacement** - Same commands, same workflow
- ⚡ **systemd powered** - Leverages Linux's native service manager
- 🚀 **Boot persistence** - Services start automatically on system boot
- 🔒 **Session independence** - Processes run without active login sessions
- 🌍 **Environment variables** - Full shell environment inheritance + custom vars
- 📊 **PM2-compatible output** - Same table and JSON formats
- 🏗️ **Ecosystem files** - Support for PM2 ecosystem.json files
- 📝 **Centralized logging** - Uses systemd's journald for log management
- 🔧 **Easy migration** - Migrate from existing PM2 setups

## Installation

### Method 1: Download Binary (Recommended)

```bash
# Download the latest release
curl -L https://github.com/wojtekw92/pm2go/releases/latest/download/pm2go-linux-amd64 -o pm2go

# Make executable
chmod +x pm2go

# Install globally
sudo mv pm2go /usr/local/bin/
```

### Method 2: Build from Source

```bash
# Clone repository
git clone https://github.com/wojtekw92/pm2go.git
cd pm2go

# Build
go build -o pm2go

# Install globally
sudo mv pm2go /usr/local/bin/
```

### Method 3: Go Install

```bash
go install github.com/wojtekw92/pm2go@latest
```

## Quick Start

### 1. Configure systemd (One-time setup)

```bash
# Configure systemd for persistent services
pm2go startup
```

This configures:
- User lingering (services persist after logout)
- Early boot startup
- systemd user instance

### 2. Start your first application

```bash
# Start a simple script
pm2go start app.js --name my-app

# Start a Python script with environment variables
pm2go start server.py --name api --env NODE_ENV=production --env PORT=8080

# Start from ecosystem file
pm2go start ecosystem.json
```

### 3. Manage your applications

```bash
# List running applications
pm2go list
pm2go ls    # short form
pm2go l     # PM2-style shortcut

# Stop an application
pm2go stop my-app

# View logs
pm2go logs my-app       # Show recent logs
pm2go logs my-app -f    # Follow logs in real-time
pm2go logs -l 100       # Show last 100 lines for all apps

# Delete an application  
pm2go delete my-app
```

## Commands Reference

### Core Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `pm2go start <script>` | | Start an application |
| `pm2go stop <name>` | | Stop an application |
| `pm2go delete <name>` | `del` | Delete an application |
| `pm2go list` | `ls`, `l` | List all applications |
| `pm2go logs [name]` | | Show application logs |

### Advanced Commands

| Command | Description |
|---------|-------------|
| `pm2go startup` | Configure systemd for boot persistence |
| `pm2go flush [name]` | Clear logs (all or specific app) |
| `pm2go jlist` | List applications in JSON format |

### Command Options

#### Start Command
```bash
pm2go start <script> [options]

Options:
  -n, --name string     Application name
  -e, --env strings     Environment variables (KEY=VALUE)
```

#### Logs Command
```bash
pm2go logs [name] [options]

Options:
  -f, --follow          Follow log output (like tail -f)
  -l, --lines int       Number of lines to display (default 50)
```

## Examples

### Basic Usage

```bash
# Start a Node.js application
pm2go start server.js --name api

# Start with custom environment
pm2go start app.py --name worker --env DEBUG=true --env WORKERS=4

# Start multiple apps from ecosystem file
pm2go start ecosystem.json

# View logs for specific application
pm2go logs api

# Follow logs in real-time
pm2go logs worker --follow

# Show last 200 lines for all applications
pm2go logs --lines 200
```

### Ecosystem File Example

Create `ecosystem.json`:

```json
{
  "apps": [
    {
      "name": "api-server",
      "script": "server.js",
      "cwd": "/var/www/api",
      "args": "--port 3000",
      "env": {
        "NODE_ENV": "production",
        "DATABASE_URL": "postgres://localhost/mydb",
        "JWT_SECRET": "your-secret-key"
      }
    },
    {
      "name": "worker",
      "script": "worker.py",
      "cwd": "/var/www/worker",
      "env": {
        "REDIS_URL": "redis://localhost:6379",
        "QUEUE_NAME": "tasks"
      }
    }
  ]
}
```

### Environment Variables

PM2go inherits all shell environment variables and allows custom ones:

```bash
# Shell variables are inherited automatically
export DATABASE_URL="postgres://localhost/mydb"
pm2go start app.js  # DATABASE_URL is available

# Add custom variables
pm2go start app.js --env NODE_ENV=production --env PORT=8080

# Variables priority: --env flags > shell environment > ecosystem file
```

## Aliasing to PM2

To use `pm2` command instead of `pm2go`:

### Option 1: Shell Alias

```bash
# Add to ~/.bashrc or ~/.zshrc
alias pm2='pm2go'

# Reload shell
source ~/.bashrc
```

### Option 2: Symbolic Link

```bash
# Create symlink
sudo ln -s /usr/local/bin/pm2go /usr/local/bin/pm2

# Now use pm2 commands
pm2 start app.js
pm2 list
```

### Option 3: Rename Binary

```bash
# Rename during installation
sudo mv pm2go /usr/local/bin/pm2
```

## Migrating from PM2

### Step 1: Export PM2 Configuration

```bash
# Export current PM2 apps to ecosystem file
pm2 ecosystem simple
# This creates ecosystem.config.js

# Convert to JSON format (manual step)
# PM2go uses ecosystem.json files
```

### Step 2: Stop PM2 Services

```bash
# Stop all PM2 processes
pm2 stop all

# Delete all PM2 processes
pm2 delete all

# Kill PM2 daemon (optional)
pm2 kill
```

### Step 3: Install and Configure PM2go

```bash
# Install PM2go
# ... (see installation section)

# Configure systemd
pm2go startup

# Alias to pm2 (optional)
alias pm2='pm2go'
```

### Step 4: Migrate Applications

#### Manual Migration

```bash
# Start each app manually
pm2go start app1.js --name app1 --env NODE_ENV=production
pm2go start app2.py --name app2 --env DEBUG=false
```

#### Ecosystem File Migration

Convert your `ecosystem.config.js` to `ecosystem.json`:

**Before (PM2):**
```javascript
module.exports = {
  apps: [{
    name: "my-app",
    script: "./app.js",
    env: {
      NODE_ENV: "development"
    },
    env_production: {
      NODE_ENV: "production"
    }
  }]
}
```

**After (PM2go):**
```json
{
  "apps": [{
    "name": "my-app",
    "script": "./app.js",
    "env": {
      "NODE_ENV": "production"
    }
  }]
}
```

Then start:
```bash
pm2go start ecosystem.json
```

### Step 5: Verify Migration

```bash
# List running services
pm2go list

# Check systemd services
systemctl --user list-units "pm2-*"

# View logs
pm2go logs app-name
pm2go logs app-name -f
```

## Log Management

PM2go uses systemd's journald for centralized logging:

### View Logs

PM2go provides a convenient logs command that uses systemd's journald:

```bash
# View logs for specific app
pm2go logs my-app

# Follow logs in real-time
pm2go logs my-app -f

# View all application logs
pm2go logs

# Show last 100 lines
pm2go logs my-app -l 100

# Advanced: Direct journalctl access
journalctl --user -u pm2-my-app -f
journalctl --user -u "pm2-*" -f
journalctl --user -u pm2-my-app --since "1 hour ago"
```

### Clear Logs

```bash
# Clear all logs
pm2go flush

# Clear specific app logs
pm2go flush my-app

# Manual journald cleanup
journalctl --user --rotate
journalctl --user --vacuum-time=1d
```

## Troubleshooting

### Common Issues

#### 1. Services don't persist after reboot

```bash
# Ensure lingering is enabled
loginctl show-user $USER | grep Linger
# Should show: Linger=yes

# If not enabled, run:
pm2go startup
```

#### 2. Permission denied errors

```bash
# Ensure proper directory permissions
mkdir -p ~/.config/systemd/user
chmod 755 ~/.config/systemd/user
```

#### 3. Services fail to start

```bash
# Check service status
systemctl --user status pm2-app-name

# View detailed logs
journalctl --user -u pm2-app-name
```

#### 4. Environment variables not working

```bash
# Check service environment
systemctl --user show pm2-app-name --property=Environment
```

### System Requirements

- **Linux with systemd** (Ubuntu 16+, CentOS 7+, Debian 9+, etc.)
- **User lingering support** (most modern distros)
- **journald** for logging (included with systemd)

### Limitations

- **Linux only** - systemd is required
- **User services** - Runs as user services (not system-wide by default)
- **No clustering** - Use systemd load balancing or external tools

## Comparison with PM2

| Feature | PM2 | PM2go |
|---------|-----|-------|
| **Process Management** | Custom daemon | systemd |
| **Boot Persistence** | Custom scripts | systemd lingering |
| **Logging** | Custom logs | journald |
| **Clustering** | Built-in | External (nginx, HAProxy) |
| **Memory Usage** | ~50MB daemon | ~0MB (uses systemd) |
| **Reliability** | Good | Excellent (systemd) |
| **Platform Support** | Cross-platform | Linux only |

## API Usage

PM2go's internal API can be used in Go applications:

```go
package main

import (
    "github.com/wojtekw92/pm2go/pkg/systemd"
)

func main() {
    // Create manager
    manager := systemd.NewManager()
    
    // Start application
    config := systemd.AppConfig{
        Name:   "my-app",
        Script: "server.js",
        Env: map[string]string{
            "NODE_ENV": "production",
            "PORT": "3000",
        },
    }
    
    err := manager.Start(config)
    if err != nil {
        panic(err)
    }
    
    // List processes
    processes, _ := manager.List()
    for _, proc := range processes {
        fmt.Printf("App: %s, Status: %s\n", proc.Name, proc.PM2Env.Status)
    }
}
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- 📖 **Documentation**: This README
- 🐛 **Bug Reports**: [GitHub Issues](https://github.com/wojtekw92/pm2go/issues)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/wojtekw92/pm2go/discussions)
- 📧 **Email**: your-email@example.com

---

**Made with ❤️ for the Linux community**