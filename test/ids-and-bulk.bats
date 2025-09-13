#!/usr/bin/env bats

# PM2go ID-based and bulk operations tests

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

@test "pm2go assigns persistent process IDs" {
    # Start first process
    run ./pm2go start test/fixtures/test-app.py --name app-1
    [[ "$status" -eq 0 ]]
    
    # Start second process
    run ./pm2go start test/fixtures/test-app.py --name app-2
    [[ "$status" -eq 0 ]]
    
    # Check list shows IDs
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"app-1"* ]]
    [[ "$output" == *"app-2"* ]]
    
    # Should show ID columns
    [[ "$output" == *"id"* ]]
}

@test "pm2go can operate on processes by ID" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-by-id
    [[ "$status" -eq 0 ]]
    
    # Stop by ID (assuming it gets ID 0)
    run ./pm2go stop 0
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Stopped"* ]]
    
    # Restart by ID
    run ./pm2go restart 0
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Restarted"* ]]
    
    # Delete by ID
    run ./pm2go delete 0
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Deleted"* ]]
}

@test "pm2go can restart all processes" {
    # Start multiple processes
    run ./pm2go start test/fixtures/test-app.py --name app-1
    [[ "$status" -eq 0 ]]
    
    run ./pm2go start test/fixtures/test-app.py --name app-2
    [[ "$status" -eq 0 ]]
    
    # Restart all
    run ./pm2go restart all
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Restarted app-1"* ]]
    [[ "$output" == *"Restarted app-2"* ]]
    [[ "$output" == *"summary"* ]]
}

@test "pm2go can stop all processes" {
    # Start multiple processes
    run ./pm2go start test/fixtures/test-app.py --name app-1
    [[ "$status" -eq 0 ]]
    
    run ./pm2go start test/fixtures/test-app.py --name app-2
    [[ "$status" -eq 0 ]]
    
    # Stop all
    run ./pm2go stop all
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Stopped app-1"* ]]
    [[ "$output" == *"Stopped app-2"* ]]
}

@test "pm2go can delete all processes" {
    # Start multiple processes
    run ./pm2go start test/fixtures/test-app.py --name app-1
    [[ "$status" -eq 0 ]]
    
    run ./pm2go start test/fixtures/test-app.py --name app-2
    [[ "$status" -eq 0 ]]
    
    # Delete all
    run ./pm2go delete all
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Deleted app-1"* ]]
    [[ "$output" == *"Deleted app-2"* ]]
    
    # Verify list is empty
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" != *"app-1"* ]]
    [[ "$output" != *"app-2"* ]]
}

@test "pm2go can restart existing process by starting with ID" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-restart-id
    [[ "$status" -eq 0 ]]
    
    # Stop it
    run ./pm2go stop test-restart-id
    [[ "$status" -eq 0 ]]
    
    # Restart using start command with ID (assuming it got ID 0)
    run ./pm2go start 0
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Restarted"* ]]
}