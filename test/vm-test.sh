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

# Build pm2go for Linux (in case it was built on different arch)
echo "Building pm2go for Linux..."
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
cp test/fixtures/* "$TEST_DIR/"
cp pm2go "$TEST_DIR/"

cd "$TEST_DIR"

echo "Test environment setup complete."
echo "Test directory: $TEST_DIR"
echo "Running E2E tests..."

# Run the VM-specific E2E tests  
cd "$PWD" # Go back to project root for go test
go test -v ./test/e2e/ -run TestVM -args -testdir="$TEST_DIR"