# PM2go Test Suite

This directory contains comprehensive tests for PM2go using the [bats-core](https://github.com/bats-core/bats-core) testing framework.

## Prerequisites

### Install bats-core

**Ubuntu/Debian:**
```bash
sudo apt-get install bats
```

**macOS:**
```bash
brew install bats-core
```

**From source:**
```bash
git clone https://github.com/bats-core/bats-core.git
cd bats-core
sudo ./install.sh /usr/local
```

### System Requirements

- Linux with systemd (for PM2go functionality)
- Python 3 (for test applications)
- Go (to build PM2go)

## Running Tests

### Run All Tests
```bash
# From project root
./test/run-tests.sh
```

### Run Individual Test Suites
```bash
# Basic operations
bats test/basic.bats

# ID-based and bulk operations
bats test/ids-and-bulk.bats

# Process inspection (describe, env commands)
bats test/inspection.bats

# Logging functionality
bats test/logging.bats

# Ecosystem file support
bats test/ecosystem.bats

# Environment variable inheritance
bats test/environment.bats

# JSON output and advanced features
bats test/json-output.bats
```

### Run Specific Tests
```bash
# Run a specific test by name
bats test/basic.bats -f "pm2go can start a simple script"

# Show test output (verbose)
bats test/basic.bats --verbose-run
```

## Test Structure

### Test Files

- **`basic.bats`** - Core functionality (start, stop, delete, list)
- **`ids-and-bulk.bats`** - Process ID operations and bulk commands (all)
- **`inspection.bats`** - Process inspection commands (describe, env)
- **`logging.bats`** - Log viewing and management
- **`ecosystem.bats`** - Ecosystem file functionality
- **`environment.bats`** - Environment variable inheritance and handling
- **`json-output.bats`** - JSON output and advanced features

### Test Fixtures

- **`fixtures/test-app.py`** - Python test application with configurable output
- **`fixtures/test-ecosystem.json`** - Sample ecosystem configuration file

## Test Application

The test suite uses a Python application (`test/fixtures/test-app.py`) that:

- Produces timestamped output every 2 seconds (configurable)
- Supports custom messages and intervals
- Can generate stderr output for testing
- Shows environment variables when requested
- Accepts various command-line arguments

### Test App Usage
```bash
# Basic usage
python3 test/fixtures/test-app.py

# Custom configuration
python3 test/fixtures/test-app.py --interval 1 --max-count 5 --message "Custom output"

# Generate errors for testing
python3 test/fixtures/test-app.py --error-every 3 --max-count 10

# Show environment variables
python3 test/fixtures/test-app.py --env-vars
```

## Test Coverage

The test suite covers:

### Core Operations
- ✅ Process start/stop/restart/delete
- ✅ Process listing with status information
- ✅ Custom interpreter and argument handling
- ✅ Error handling for invalid operations

### Advanced Features
- ✅ Persistent process IDs
- ✅ ID-based operations (`pm2go logs 0`, `pm2go restart 1`)
- ✅ Bulk operations (`pm2go restart all`, `pm2go delete all`)
- ✅ Process inspection (`describe`, `env` commands)

### Environment Variables
- ✅ Complete environment inheritance
- ✅ Variables with spaces and special characters
- ✅ Unicode character support
- ✅ Command-line variable overrides

### Logging
- ✅ PM2-style file-based logging
- ✅ Log file creation and structure
- ✅ Log viewing by name and ID
- ✅ Combined stdout/stderr output
- ✅ Log following (`-f` flag)

### Ecosystem Files
- ✅ JSON ecosystem file parsing
- ✅ Multiple application startup
- ✅ Per-app configuration (interpreter, args, env)
- ✅ Individual management of ecosystem apps

### JSON Output
- ✅ PM2-compatible JSON format (`jlist` command)
- ✅ Process monitoring data (CPU, memory)
- ✅ Unicode-aware table formatting

## Test Philosophy

The tests follow these principles:

1. **Isolation** - Each test cleans up after itself
2. **Real-world scenarios** - Tests use actual system processes
3. **Comprehensive coverage** - All major features are tested
4. **Fast execution** - Tests use short-lived processes where possible
5. **Clear assertions** - Tests verify specific, observable behaviors

## Troubleshooting

### Common Issues

**Tests fail with "systemd not available":**
- Ensure you're running on a Linux system with systemd
- Check that user services are supported: `systemctl --user status`

**Permission errors:**
- Ensure proper directory permissions: `chmod 755 ~/.config/systemd/user`
- Check systemd user lingering: `loginctl show-user $USER | grep Linger`

**Python test app fails:**
- Ensure Python 3 is installed: `python3 --version`
- Check file permissions: `chmod +x test/fixtures/test-app.py`

**Tests hang or timeout:**
- Some tests wait for process output - this is normal
- Check for orphaned processes: `./pm2go list`
- Clean up manually if needed: `./pm2go delete all`

### Debug Mode

Run individual tests with verbose output:
```bash
bats test/basic.bats --verbose-run -f "specific test name"
```

Check PM2go process status during tests:
```bash
# In another terminal
watch -n 1 './pm2go list'
```

## Contributing

When adding new tests:

1. Follow existing naming conventions
2. Include proper setup/teardown
3. Test both success and error cases
4. Use descriptive test names
5. Add tests to the appropriate file based on functionality
6. Update this README if adding new test files

### Test Naming Convention

```bash
@test "pm2go <command> <specific behavior>"
```

Examples:
- `@test "pm2go can start a simple script"`
- `@test "pm2go describe shows detailed process information"`
- `@test "pm2go restart all processes"`