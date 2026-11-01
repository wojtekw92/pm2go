# PM2go - PM2 Usage Compatibility on systemd

PM2go translates a documented subset of PM2 commands and ecosystem configuration into native systemd units and operations. It is designed for Linux operators and existing scripts that need familiar PM2 workflows without a separate PM2 process-manager daemon.

Systemd is the source of truth for process definitions, runtime state, restart supervision, signals, and startup membership. See [Architecture](docs/architecture.md) and [PM2 Usage Compatibility](docs/compatibility.md).

## Features

- 🔄 **Documented PM2 compatibility** - Familiar core commands and a strict ecosystem JSON subset
- ⚡ **systemd powered** - Leverages Linux's native service manager
- 🚀 **Native boot persistence** - `save` records online units in a systemd target
- 🔒 **Session independence** - Processes run without active login sessions
- 🌍 **Complete environment support** - Full shell environment inheritance with proper quoting
- 📊 **PM2-compatible JSON fields** - Includes PM2 IDs, timestamps, modes, and argument arrays
- 🏗️ **Ecosystem files** - Support for PM2 ecosystem.json files
- 📝 **PM2-style logging** - File-based logs in `~/.pm2/logs/` directory
- 🔧 **Migration path** - Validate existing scripts against the compatibility matrix
- 🆔 **Persistent process IDs** - Consistent IDs across restarts
- 🔍 **Advanced process inspection** - `describe` and `env` commands
- ↩️ **Smart restart handling** - Supports both individual and bulk operations
- 🎨 **Unicode-aware tables** - Proper handling of international characters
- 📈 **On-demand monitoring** - CPU, memory, PID, status, and uptime reporting

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

# Start a Python script with custom interpreter and arguments
export TEST_ENV="hello world"
pm2go start server.py --name api --interpreter python3 -- --port 8080

# All environment variables are inherited automatically
pm2go start app.js --name webapp --interpreter node

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

# Save currently online applications for boot and resurrect
pm2go save
```

## Commands Reference

### Core Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `pm2go start <script> [-- app args]` | | Start an application |
| `pm2go stop <name\|id\|all>` | | Stop applications |
| `pm2go restart <name\|id\|all>` | | Restart applications |
| `pm2go delete <name\|id\|all>` | `del` | Delete applications |
| `pm2go list` | `ls`, `l` | List all applications with CPU/memory |
| `pm2go logs [name\|id]` | | Show application logs from files |

### Inspection Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `pm2go describe <name\|id>` | `desc`, `show` | Show detailed process information |
| `pm2go env <name\|id>` | | Show process environment variables |

### Advanced Commands

| Command | Description |
|---------|-------------|
| `pm2go startup` | Configure systemd for boot persistence |
| `pm2go save` | Save online units in the PM2go systemd target |
| `pm2go resurrect` | Start existing units in the saved target |
| `pm2go flush [name]` | Clear logs (all or specific app) |
| `pm2go jlist` | List applications in JSON format |

### Command Options

#### Start Command
```bash
# Simple script
pm2go start <script> [options]

# Custom interpreter with arguments
pm2go start <script> --interpreter <interpreter> -- [script-args]

# Restart existing process by ID
pm2go start <id>

Options:
  -n, --name string          Application name
      --env strings          Ecosystem environment or KEY=VALUE override
      --interpreter string   Application interpreter
      --cwd string           Working directory
  -o, --output string        Standard output log path
  -e, --error string         Standard error log path
  -l, --log string           Combined log path
```

#### Logs Command
```bash
pm2go logs [name|id] [options]

Options:
  -f, --follow          Follow log output (like tail -f)
  -l, --lines int       Number of lines to display (default 50)
```

#### Restart Command
```bash
pm2go restart <name|id|all>

Examples:
  pm2go restart my-app    # Restart by name
  pm2go restart 0         # Restart by ID
  pm2go restart all       # Restart all processes
```

## Examples

### Basic Usage

```bash
# Start a Node.js application
pm2go start server.js --name api

# Start Python script with custom interpreter and arguments
pm2go start app.py --name worker --interpreter python3 -- --port 8000 --workers 4

# Start from ecosystem file
pm2go start ecosystem.json

# List all processes with CPU and memory usage
pm2go list

# Restart all processes
pm2go restart all

# View detailed process information
pm2go describe api
pm2go describe 0        # By ID

# Show process environment variables
pm2go env worker
pm2go env 1             # By ID

# View logs by name or ID
pm2go logs api
pm2go logs 0            # By ID

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
      "interpreter": "node",
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
      "interpreter": "/usr/bin/python3",
      "cwd": "/var/www/worker",
      "args": "--workers 4",
      "env": {
        "REDIS_URL": "redis://localhost:6379",
        "QUEUE_NAME": "tasks"
      }
    }
  ]
}
```

### Environment Variables

PM2go automatically inherits **ALL** shell environment variables with proper quoting support:

```bash
# All shell variables are inherited automatically (including spaces and special chars)
export DATABASE_URL="postgres://user:pass@localhost/mydb"
export DEBUG_MESSAGE="Hello world with spaces"
export API_KEYS='["key1", "key2"]'
pm2go start app.js  # All variables available with proper quoting

# Variables are properly escaped in systemd service files
pm2go describe my-app    # Shows all environment variables
pm2go env my-app         # Lists just the environment variables

# Command-line variables override shell variables
pm2go start app.js --env NODE_ENV=production --env DEBUG_MESSAGE="Override value"
```

### Advanced Process Management

```bash
# Start with custom interpreter and complex arguments
pm2go start train.py --name ml-worker --interpreter python3 -- --model transformer --batch-size 32

# Process inspection
pm2go describe ml-worker     # Detailed process info
pm2go env ml-worker          # Environment variables only

# Bulk operations
pm2go restart all           # Restart all processes
pm2go stop all              # Stop all processes  
pm2go delete all            # Delete all processes

# ID-based operations (processes have persistent IDs)
pm2go start 0               # Restart process with ID 0
pm2go logs 1 -f            # Follow logs for process ID 1
pm2go describe 2           # Describe process ID 2
```

## Using PM2go From Existing Scripts

Do not globally alias or symlink `pm2` to `pm2go` until every command and
option used by your automation is listed in
[the compatibility contract](docs/compatibility.md). PM2go intentionally
rejects unsupported behavior rather than silently approximating it.

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

PM2go uses **PM2-style file-based logging** with logs stored in `~/.pm2/logs/`:

### View Logs

PM2go currently provides partial PM2-style logging with file-based storage:

```bash
# View logs for specific app (reads from files)
pm2go logs my-app
pm2go logs 0            # By process ID

# Follow logs in real-time
pm2go logs my-app -f
pm2go logs 0 -f         # By process ID

# View all application logs
pm2go logs

# Show last 100 lines
pm2go logs my-app -l 100

# Log files are stored in PM2-style structure:
# ~/.pm2/logs/my-app-out.log    (stdout)
# ~/.pm2/logs/my-app-error.log  (stderr)
```

### Log File Structure

```bash
# PM2-compatible log directory structure
~/.pm2/logs/
├── app-out.log           # app stdout
├── app-error.log         # app stderr
├── worker-out.log        # worker stdout
├── worker-error.log      # worker stderr
└── ...

# Direct file access (if needed)
tail -f ~/.pm2/logs/my-app-out.log
tail -f ~/.pm2/logs/my-app-error.log
```

### Clear Logs

```bash
# Clear all logs
pm2go flush

# Clear specific app logs  
pm2go flush my-app

# Manual log cleanup
rm ~/.pm2/logs/*
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

- **Linux with a tested systemd version supporting generated directives such as `StandardOutput=append:`**
- **User lingering support** (most modern distros)
- **systemd user or system manager access**

### Limitations

- **Linux only** - systemd is required
- **User services** - Runs as user services (not system-wide by default)
- **No clustering** - Use systemd load balancing or external tools
- **No Node.js PM2 module API** - PM2go provides a Go API and CLI, not `require("pm2")`
- **JSON ecosystem files only** - JavaScript, YAML, cluster, watch, and deploy configurations are not yet supported

## Comparison with PM2

| Feature | PM2 | PM2go |
|---------|-----|-------|
| **Process Management** | Custom daemon | systemd |
| **Boot Persistence** | PM2 dump and startup service | Saved systemd target membership |
| **Logging** | File-based (~/.pm2/logs/) | File-based (~/.pm2/logs/) |
| **Process IDs** | Sequential | Persistent across restarts |
| **Environment Variables** | Shell inheritance and ecosystem overrides | Shell inheritance and ecosystem overrides |
| **CPU/Memory Monitoring** | Built-in | On-demand, currently via `/proc` |
| **Clustering** | Built-in | External (nginx, HAProxy) |
| **Local Supervisor** | PM2 daemon | systemd; no PM2go application daemon |
| **Platform Support** | Cross-platform | Linux only |
| **Unicode Support** | Basic | Full Unicode table rendering |

## API Usage

PM2go's internal API can be used in Go applications:

```go
package main

import (
    "fmt"

    "github.com/wojtekw92/pm2go/pkg/systemd"
)

func main() {
    // Create manager
    manager := systemd.NewManager()
    
    // Start application
    config := systemd.AppConfig{
        ID:          0,
        Name:        "my-app",
        Script:      "server.js",
        Interpreter: "node",
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
    processes, err := manager.List()
    if err != nil {
        panic(err)
    }
    for _, proc := range processes {
        fmt.Printf("App: %s, Status: %s, CPU: %d%%, Memory: %s\n", 
            proc.Name, proc.PM2Env.Status, proc.Monit.CPU, proc.Monit.Memory)
    }
}
```

## New Features in This Version

### 🎯 Enhanced Process Management
- **Persistent Process IDs**: Process IDs remain consistent across restarts
- **Bulk Operations**: `restart all`, `stop all`, `delete all` commands
- **ID-Based Operations**: All commands now accept process IDs: `pm2go logs 0`, `pm2go restart 1`

### 🔍 Advanced Process Inspection
- **`describe` command**: Detailed process information with PM2-style formatting
- **`env` command**: View all environment variables for any process
- **Divergent environment display**: See which variables differ from current shell

### ⚙️ Better Interpreter Support
- **Custom interpreter paths**: `pm2go start script.py --name app --interpreter python3`
- **Full path resolution**: Works with `nvm`, `venv`, and other environment managers
- **Argument handling**: Proper support for complex command-line arguments

### 📊 On-Demand Monitoring
- **CPU usage calculation**: Lifetime-average CPU estimate from `/proc/pid/stat`
- **Memory tracking**: Main-process resident memory display
- **Enhanced process table**: Dynamic column sizing with Unicode support

### 🌐 Complete Environment Support
- **Full environment inheritance**: ALL shell variables are passed through
- **Proper quoting**: Handles spaces, special characters, and quotes in values
- **systemd integration**: Environment variables properly escaped in service files

### 📝 PM2-Style Logging
- **File-based logs**: Stores logs in `~/.pm2/logs/` like PM2
- **ID-based log access**: `pm2go logs 0` works with process IDs
- **Real-time following**: `pm2go logs app -f` for live log streaming

## Contributing

Read [AGENTS.md](AGENTS.md), [Architecture](docs/architecture.md), the
[compatibility contract](docs/compatibility.md), and the
[roadmap](docs/roadmap.md) before changing lifecycle or compatibility behavior.

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
