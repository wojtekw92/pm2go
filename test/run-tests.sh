#!/bin/bash
set -e

# PM2go Test Runner
# Requires bats-core to be installed

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "=== PM2go Test Suite ==="
echo "Running tests with bats-core..."
echo

# Check if bats is available
if ! command -v bats &> /dev/null; then
    echo "ERROR: bats-core is not installed"
    echo
    echo "Install bats-core:"
    echo "  Ubuntu/Debian: sudo apt-get install bats"
    echo "  macOS: brew install bats-core"
    echo "  From source: https://github.com/bats-core/bats-core"
    echo
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "ERROR: Go is not installed or not on PATH"
    exit 1
fi

export PM2GO_SERVICE_PREFIX="${PM2GO_SERVICE_PREFIX:-pm2go-test-$(date +%s)-}"

cleanup() {
    ./pm2go delete all 2>/dev/null || true
}
trap cleanup EXIT

echo "Building pm2go..."
go build -o pm2go .
echo

echo "Using test service prefix: $PM2GO_SERVICE_PREFIX"
echo "Cleaning up existing test PM2go processes..."
cleanup
echo

# Run the test suites
echo "Running test suites..."
echo

# Basic functionality tests
echo "1. Basic Operations Tests"
bats test/basic.bats

echo
echo "2. ID-based and Bulk Operations Tests"
bats test/ids-and-bulk.bats

echo
echo "3. Process Inspection Tests"
bats test/inspection.bats

echo
echo "4. Logging Tests"
bats test/logging.bats

echo
echo "5. Ecosystem File Tests"
bats test/ecosystem.bats

echo
echo "6. Environment Variables Tests"
bats test/environment.bats

echo
echo "7. JSON Output and Advanced Features Tests"
bats test/json-output.bats

echo
echo "=== Test Suite Complete ==="

echo "Cleaning up test processes..."
cleanup

echo
echo "All tests completed successfully!"
