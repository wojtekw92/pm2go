#!/usr/bin/env bats

# Basic PM2go functionality tests

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

@test "pm2go version shows help" {
    run ./pm2go --help
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"PM2 Systemd Wrapper"* ]]
}

@test "pm2go list shows empty list initially" {
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    # Should show table headers but no processes
    [[ "$output" == *"name"* ]]
    [[ "$output" == *"status"* ]]
}

@test "pm2go can start a simple script" {
    run ./pm2go start test/fixtures/test-app.py --name test-simple
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Started test-simple"* ]]
    
    # Verify it appears in list
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"test-simple"* ]]
}

@test "pm2go can start with custom interpreter and args" {
    run ./pm2go start python3 --name test-custom -- test/fixtures/test-app.py --interval 1 --max-count 3
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Started test-custom"* ]]
    
    # Wait a moment for the process to start
    sleep 1
    
    # Check if it's running
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"test-custom"* ]]
}

@test "pm2go can stop a process by name" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-stop
    [[ "$status" -eq 0 ]]
    
    # Stop it
    run ./pm2go stop test-stop
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Stopped test-stop"* ]]
    
    # Verify it's stopped
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"stopped"* ]]
}

@test "pm2go can restart a process" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-restart
    [[ "$status" -eq 0 ]]
    
    # Restart it
    run ./pm2go restart test-restart
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Restarted test-restart"* ]]
}

@test "pm2go can delete a process" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-delete
    [[ "$status" -eq 0 ]]
    
    # Delete it
    run ./pm2go delete test-delete
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Deleted test-delete"* ]]
    
    # Verify it's gone from list
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" != *"test-delete"* ]]
}

@test "pm2go shows error for non-existent process" {
    run ./pm2go stop non-existent
    [[ "$status" -eq 1 ]]
    [[ "$output" == *"not found"* ]]
}