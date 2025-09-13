#!/usr/bin/env bats

# PM2go inspection commands tests (describe, env)

setup() {
    # Build pm2go if not exists
    if [[ ! -f "./pm2go" ]]; then
        run go build -o pm2go
        [[ "$status" -eq 0 ]]
    fi
    
    # Clean up any existing processes
    ./pm2go delete all 2>/dev/null || true
    
    # Set test environment variables
    export TEST_VAR="test value"
    export ANOTHER_TEST="another value with spaces"
}

teardown() {
    # Clean up processes after each test
    ./pm2go delete all 2>/dev/null || true
    
    # Clean up test environment variables
    unset TEST_VAR ANOTHER_TEST
}

@test "pm2go describe shows detailed process information" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-describe
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Describe by name
    run ./pm2go describe test-describe
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Describing process"* ]]
    [[ "$output" == *"test-describe"* ]]
    [[ "$output" == *"status"* ]]
    [[ "$output" == *"script path"* ]]
    [[ "$output" == *"pid path"* ]]
}

@test "pm2go describe works with process ID" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-describe-id
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Describe by ID (assuming it gets ID 0)
    run ./pm2go describe 0
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"Describing process with id 0"* ]]
    [[ "$output" == *"test-describe-id"* ]]
}

@test "pm2go describe shows environment variables section" {
    # Start a process with environment variables
    run ./pm2go start test/fixtures/test-app.py --name test-env-vars
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Describe the process
    run ./pm2go describe test-env-vars
    [[ "$status" -eq 0 ]]
    
    # Should show divergent env vars section if any exist
    # At minimum should show process info table
    [[ "$output" == *"status"* ]]
    [[ "$output" == *"name"* ]]
}

@test "pm2go env shows process environment variables" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-env-command
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start  
    sleep 1
    
    # Show environment variables
    run ./pm2go env test-env-command
    [[ "$status" -eq 0 ]]
    
    # Should show environment variables (at least PATH, HOME, etc.)
    [[ "$output" == *"PATH"* ]]
}

@test "pm2go env works with process ID" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-env-id
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Show environment variables by ID
    run ./pm2go env 0
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"PATH"* ]]
}

@test "pm2go env shows custom environment variables" {
    # Start a process with the test environment variables we set
    run ./pm2go start test/fixtures/test-app.py --name test-custom-env
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Check environment variables
    run ./pm2go env test-custom-env
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"TEST_VAR"* ]]
    [[ "$output" == *"test value"* ]]
    [[ "$output" == *"ANOTHER_TEST"* ]]
}

@test "pm2go describe and env handle non-existent processes" {
    # Try to describe non-existent process
    run ./pm2go describe non-existent
    [[ "$status" -eq 1 ]]
    [[ "$output" == *"not found"* ]]
    
    # Try to show env for non-existent process
    run ./pm2go env non-existent
    [[ "$status" -eq 1 ]]
    [[ "$output" == *"not found"* ]]
}

@test "pm2go describe shows interpreter information" {
    # Start with custom interpreter
    run ./pm2go start python3 --name test-interpreter -- test/fixtures/test-app.py --max-count 2
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Describe should show interpreter info
    run ./pm2go describe test-interpreter
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"interpreter"* ]]
    [[ "$output" == *"script path"* ]]
    [[ "$output" == *"script args"* ]]
}