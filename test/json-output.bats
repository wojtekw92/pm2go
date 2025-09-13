#!/usr/bin/env bats

# PM2go JSON output and advanced features tests

setup() {
    # Build pm2go if not exists
    if [[ ! -f "./pm2go" ]]; then
        run go build -o pm2go
        [[ "$status" -eq 0 ]]
    fi
    
    # Clean up any existing processes
    ./pm2go delete all 2>/dev/null || true
}

teardown() {
    # Clean up processes after each test
    ./pm2go delete all 2>/dev/null || true
}

@test "pm2go jlist produces valid JSON output" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-json
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Get JSON list
    run ./pm2go jlist
    [[ "$status" -eq 0 ]]
    
    # Output should be valid JSON (basic check)
    [[ "$output" == *"["* ]]
    [[ "$output" == *"]"* ]]
    [[ "$output" == *"test-json"* ]]
    [[ "$output" == *"pm2_env"* ]]
}

@test "pm2go jlist shows empty array for no processes" {
    # Get JSON list with no processes
    run ./pm2go jlist
    [[ "$status" -eq 0 ]]
    
    # Should return empty JSON array
    [[ "$output" == "[]" ]]
}

@test "pm2go jlist shows multiple processes" {
    # Start multiple processes
    run ./pm2go start test/fixtures/test-app.py --name app-1
    [[ "$status" -eq 0 ]]
    
    run ./pm2go start test/fixtures/test-app.py --name app-2
    [[ "$status" -eq 0 ]]
    
    # Wait for processes to start
    sleep 1
    
    # Get JSON list
    run ./pm2go jlist
    [[ "$status" -eq 0 ]]
    
    # Should contain both processes
    [[ "$output" == *"app-1"* ]]
    [[ "$output" == *"app-2"* ]]
    [[ "$output" == *"pm2_env"* ]]
    [[ "$output" == *"monit"* ]]
}

@test "pm2go jlist includes monitoring data" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-monitoring
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start and generate some activity
    sleep 2
    
    # Get JSON list
    run ./pm2go jlist
    [[ "$status" -eq 0 ]]
    
    # Should include monitoring information
    [[ "$output" == *"monit"* ]]
    [[ "$output" == *"memory"* ]]
    [[ "$output" == *"cpu"* ]]
}

@test "pm2go list shows table with CPU and memory columns" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-table
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Get table list
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    
    # Should show table headers including CPU and memory
    [[ "$output" == *"cpu"* ]]
    [[ "$output" == *"memory"* ]]
    [[ "$output" == *"pid"* ]]
    [[ "$output" == *"status"* ]]
    [[ "$output" == *"test-table"* ]]
}

@test "pm2go list handles unicode characters properly" {
    # Start a process with unicode in name
    run ./pm2go start test/fixtures/test-app.py --name "test-café"
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Get list - should handle unicode properly
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"test-café"* ]]
    
    # Table should be properly formatted
    [[ "$output" == *"┌"* ]]
    [[ "$output" == *"└"* ]]
}

@test "pm2go startup command exists" {
    # Test that startup command exists and provides help
    run ./pm2go startup --help
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"startup"* ]]
}

@test "pm2go shows help for unknown command" {
    # Test unknown command
    run ./pm2go unknown-command
    [[ "$status" -eq 1 ]]
    [[ "$output" == *"unknown command"* ]] || [[ "$output" == *"Error"* ]]
}

@test "pm2go handles empty command gracefully" {
    # Test with no arguments
    run ./pm2go
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"PM2 Systemd Wrapper"* ]]
}

@test "pm2go list shows stopped processes" {
    # Start and then stop a process
    run ./pm2go start test/fixtures/test-app.py --name test-stopped
    [[ "$status" -eq 0 ]]
    
    # Stop the process
    run ./pm2go stop test-stopped
    [[ "$status" -eq 0 ]]
    
    # List should still show the stopped process
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"test-stopped"* ]]
    [[ "$output" == *"stopped"* ]]
}