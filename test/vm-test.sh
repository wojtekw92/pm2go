#!/bin/bash

# VM-based E2E test runner for pm2go
# This script should be run inside your UTM Linux VM

set -e

echo "PM2go E2E Tests - VM Environment"
echo "================================="

# Check if we're in a proper Linux environment
if [[ "$(uname)" != "Linux" ]]; then
    echo "Error: This test must be run in a Linux environment (UTM VM)"
    exit 1
fi

# Check if systemd is available
if ! command -v systemctl &> /dev/null; then
    echo "Error: systemctl not found. This test requires systemd."
    exit 1
fi

# Find project root (where go.mod is located)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Project root: $PROJECT_ROOT"

# Build pm2go for Linux (in case it was built on different arch)
echo "Building pm2go for Linux..."
cd "$PROJECT_ROOT"
if [[ -f "go.mod" ]]; then
    go build -o pm2go .
else
    echo "Error: go.mod not found. Are you in the project root?"
    exit 1
fi

# Make pm2go executable
chmod +x pm2go

# Copy test fixtures to temporary directory
TEST_DIR="/tmp/pm2go-test"
mkdir -p "$TEST_DIR"
cp "$PROJECT_ROOT"/test/fixtures/* "$TEST_DIR/"
cp "$PROJECT_ROOT"/pm2go "$TEST_DIR/"

# Update ecosystem.json to use correct test directory paths
sed -i "s|/home/ubuntu/apps|$TEST_DIR|g" "$TEST_DIR/ecosystem.json"

echo "Test environment setup complete."
echo "Test directory: $TEST_DIR"
echo "Running E2E tests..."

# Run the VM-specific E2E tests  
cd "$PROJECT_ROOT/test/e2e" # Go to e2e directory (has its own go.mod)
go test -v . -run TestVM -args -testdir="$TEST_DIR"