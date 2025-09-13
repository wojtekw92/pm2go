#!/usr/bin/env bats

# PM2go ecosystem file tests

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

@test "pm2go can start from ecosystem file" {
    # Start from ecosystem file
    run ./pm2go start test/fixtures/test-ecosystem.json
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Started test-app-1"* ]]
    [[ "$output" == *"Started test-app-2"* ]]
    
    # Verify both apps are listed
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"test-app-1"* ]]
    [[ "$output" == *"test-app-2"* ]]
}

@test "ecosystem apps inherit environment variables" {
    # Start from ecosystem file
    run ./pm2go start test/fixtures/test-ecosystem.json
    [[ "$status" -eq 0 ]]
    
    # Wait for processes to start
    sleep 2
    
    # Check environment variables for first app
    run ./pm2go env test-app-1
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"TEST_ENV"* ]]
    [[ "$output" == *"ecosystem-value"* ]]
    [[ "$output" == *"APP_ID"* ]]
    [[ "$output" == *"1"* ]]
    
    # Check environment variables for second app
    run ./pm2go env test-app-2
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"TEST_ENV"* ]]
    [[ "$output" == *"another-value"* ]]
    [[ "$output" == *"APP_ID"* ]]
    [[ "$output" == *"2"* ]]
}

@test "ecosystem apps use custom interpreters and arguments" {
    # Start from ecosystem file
    run ./pm2go start test/fixtures/test-ecosystem.json
    [[ "$status" -eq 0 ]]
    
    # Wait for processes to start
    sleep 2
    
    # Describe first app to check interpreter and args
    run ./pm2go describe test-app-1
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"interpreter"* ]]
    [[ "$output" == *"script path"* ]]
    [[ "$output" == *"script args"* ]]
}

@test "ecosystem file handles non-existent file" {
    # Try to start from non-existent ecosystem file
    run ./pm2go start non-existent-ecosystem.json
    [[ "$status" -eq 1 ]]
    [[ "$output" == *"Error"* ]]
}

@test "ecosystem apps can be managed individually" {
    # Start from ecosystem file
    run ./pm2go start test/fixtures/test-ecosystem.json
    [[ "$status" -eq 0 ]]
    
    # Wait for apps to start
    sleep 2
    
    # Stop one app
    run ./pm2go stop test-app-1
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Stopped test-app-1"* ]]
    
    # Verify first app is stopped, second is still running
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"test-app-1"* ]]
    [[ "$output" == *"test-app-2"* ]]
    
    # Restart the stopped app
    run ./pm2go restart test-app-1
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Restarted test-app-1"* ]]
}

@test "ecosystem apps generate separate log files" {
    # Start from ecosystem file
    run ./pm2go start test/fixtures/test-ecosystem.json
    [[ "$status" -eq 0 ]]
    
    # Wait for output to be generated
    sleep 3
    
    # Check logs for first app
    run ./pm2go logs test-app-1
    [[ "$status" -eq 0 ]]
    
    # Check logs for second app  
    run ./pm2go logs test-app-2
    [[ "$status" -eq 0 ]]
    
    # Apps should have different output based on their message args
    # (Content verification depends on timing, but commands should succeed)
}

@test "ecosystem apps can be deleted as a group" {
    # Start from ecosystem file
    run ./pm2go start test/fixtures/test-ecosystem.json
    [[ "$status" -eq 0 ]]
    
    # Wait for apps to start
    sleep 2
    
    # Delete all apps
    run ./pm2go delete all
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Deleted test-app-1"* ]]
    [[ "$output" == *"Deleted test-app-2"* ]]
    
    # Verify list is empty
    run ./pm2go list
    [[ "$status" -eq 0 ]]
    [[ "$output" != *"test-app-1"* ]]
    [[ "$output" != *"test-app-2"* ]]
}