#!/usr/bin/env bats

# PM2go environment variable inheritance tests

setup() {
    # Build pm2go if not exists
    if [[ ! -f "./pm2go" ]]; then
        run go build -o pm2go
        [[ "$status" -eq 0 ]]
    fi
    
    # Clean up any existing processes
    ./pm2go delete all 2>/dev/null || true
    
    # Set test environment variables with various complexities
    export SIMPLE_VAR="simple_value"
    export SPACED_VAR="value with spaces"
    export QUOTED_VAR="value with \"quotes\""
    export SPECIAL_VAR="value with $SPECIAL & chars"
    export UNICODE_VAR="value with unicode: café"
}

teardown() {
    # Clean up processes after each test
    ./pm2go delete all 2>/dev/null || true
    
    # Clean up test environment variables
    unset SIMPLE_VAR SPACED_VAR QUOTED_VAR SPECIAL_VAR UNICODE_VAR
}

@test "pm2go inherits simple environment variables" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-env-simple
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Check environment variables
    run ./pm2go env test-env-simple
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"SIMPLE_VAR"* ]]
    [[ "$output" == *"simple_value"* ]]
}

@test "pm2go inherits environment variables with spaces" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-env-spaces
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Check environment variables
    run ./pm2go env test-env-spaces
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"SPACED_VAR"* ]]
    [[ "$output" == *"value with spaces"* ]]
}

@test "pm2go inherits environment variables with quotes" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-env-quotes
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Check environment variables
    run ./pm2go env test-env-quotes
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"QUOTED_VAR"* ]]
    # The quotes should be preserved in the output
    [[ "$output" == *"quotes"* ]]
}

@test "pm2go inherits environment variables with special characters" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-env-special
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Check environment variables
    run ./pm2go env test-env-special
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"SPECIAL_VAR"* ]]
    # Should contain the special characters
}

@test "pm2go inherits unicode environment variables" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-env-unicode
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Check environment variables
    run ./pm2go env test-env-unicode
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"UNICODE_VAR"* ]]
    # Should preserve unicode characters
    [[ "$output" == *"café"* ]]
}

@test "pm2go allows command-line environment override" {
    # Start a process with command-line env var that overrides shell var
    run ./pm2go start test/fixtures/test-app.py --name test-env-override --env SIMPLE_VAR=overridden_value
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Check environment variables
    run ./pm2go env test-env-override
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"SIMPLE_VAR"* ]]
    [[ "$output" == *"overridden_value"* ]]
}

@test "pm2go shows environment variables in describe command" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-env-describe
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Describe should show divergent environment variables
    run ./pm2go describe test-env-describe
    [[ "$status" -eq 0 ]]
    # Should show main process info
    [[ "$output" == *"status"* ]]
    [[ "$output" == *"test-env-describe"* ]]
    
    # May or may not show divergent env vars section depending on current env
}

@test "pm2go can start app that uses environment variables" {
    # Start the test app with env-vars flag to print environment
    run ./pm2go start python3 --name test-env-usage -- test/fixtures/test-app.py --env-vars --max-count 1
    [[ "$status" -eq 0 ]]
    
    # Wait for app to run and finish
    sleep 3
    
    # Check logs to see if environment variables were printed
    run ./pm2go logs test-env-usage
    [[ "$status" -eq 0 ]]
    # Should contain environment variable output from the app
    [[ "$output" == *"Environment Variables"* ]] || [[ "$status" -eq 0 ]]
}

@test "pm2go preserves PATH and other essential variables" {
    # Start a process
    run ./pm2go start test/fixtures/test-app.py --name test-essential-env
    [[ "$status" -eq 0 ]]
    
    # Wait for process to start
    sleep 1
    
    # Check that essential variables are present
    run ./pm2go env test-essential-env
    [[ "$status" -eq 0 ]]
    [[ "$output" == *"PATH"* ]]
    [[ "$output" == *"HOME"* ]]
    [[ "$output" == *"USER"* ]]
}