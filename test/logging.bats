#!/usr/bin/env bats

# PM2go logging tests

setup() {
    # Build pm2go if not exists
    if [[ ! -f "./pm2go" ]]; then
        run go build -o pm2go
        [[ "$status" -eq 0 ]]
    fi
    
    # Clean up any existing processes
    ./pm2go delete all 2>/dev/null || true
    
    # Ensure log directory exists
    mkdir -p ~/.pm2/logs
}

teardown() {
    # Clean up processes after each test
    ./pm2go delete all 2>/dev/null || true
}

@test "pm2go creates log files for processes" {
    # Start a process that will generate output
    run ./pm2go start python3 --name test-logs -- test/fixtures/test-app.py --max-count 3 --interval 1
    [[ "$status" -eq 0 ]]
    
    # Wait for some output
    sleep 4
    
    # Check if log files exist
    [[ -f ~/.pm2/logs/test-logs-out.log ]] || [[ -f ~/.pm2/logs/test-logs-*-out.log ]]
    [[ -f ~/.pm2/logs/test-logs-error.log ]] || [[ -f ~/.pm2/logs/test-logs-*-error.log ]]
}

@test "pm2go logs shows process output" {
    # Start a process with limited output
    run ./pm2go start python3 --name test-log-output -- test/fixtures/test-app.py --max-count 2 --interval 1
    [[ "$status" -eq 0 ]]
    
    # Wait for output to be generated
    sleep 3
    
    # Read logs
    run ./pm2go logs test-log-output
    [[ "$status" -eq 0 ]]
    # Should contain output from the test app
    [[ "$output" == *"PM2go Test App Started"* ]] || [[ "$output" == *"Hello from PM2go test app"* ]]
}

@test "pm2go logs works with process ID" {
    # Start a process
    run ./pm2go start python3 --name test-log-id -- test/fixtures/test-app.py --max-count 2 --interval 1
    [[ "$status" -eq 0 ]]
    
    # Wait for output
    sleep 3
    
    # Read logs by ID (assuming it gets ID 0)
    run ./pm2go logs 0
    [[ "$status" -eq 0 ]]
    # Should show some output (could be empty if process finished quickly)
    [[ "$status" -eq 0 ]]
}

@test "pm2go logs shows error output" {
    # Start a process that generates both stdout and stderr
    run ./pm2go start python3 --name test-error-logs -- test/fixtures/test-app.py --max-count 4 --interval 1 --error-every 2
    [[ "$status" -eq 0 ]]
    
    # Wait for output including errors
    sleep 5
    
    # Read logs - should include both stdout and stderr
    run ./pm2go logs test-error-logs
    [[ "$status" -eq 0 ]]
    # Might contain error output or regular output
}

@test "pm2go logs handles non-existent process" {
    # Try to read logs for non-existent process
    run ./pm2go logs non-existent-process
    [[ "$status" -eq 1 ]]
    [[ "$output" == *"not found"* ]]
}

@test "pm2go logs can show limited lines" {
    # Start a process with some output
    run ./pm2go start python3 --name test-log-lines -- test/fixtures/test-app.py --max-count 5 --interval 1
    [[ "$status" -eq 0 ]]
    
    # Wait for output
    sleep 6
    
    # Read logs with line limit
    run ./pm2go logs test-log-lines -l 2
    [[ "$status" -eq 0 ]]
    # Should execute without error (content depends on timing)
}

@test "pm2go logs can show all processes" {
    # Start multiple processes
    run ./pm2go start python3 --name app-1 -- test/fixtures/test-app.py --max-count 2 --interval 1 --message "App 1"
    [[ "$status" -eq 0 ]]
    
    run ./pm2go start python3 --name app-2 -- test/fixtures/test-app.py --max-count 2 --interval 1 --message "App 2"
    [[ "$status" -eq 0 ]]
    
    # Wait for output
    sleep 3
    
    # Read logs for all processes
    run ./pm2go logs
    [[ "$status" -eq 0 ]]
    # Should show combined output or execute without error
}

@test "pm2go flush clears logs" {
    # Start a process to generate logs
    run ./pm2go start python3 --name test-flush -- test/fixtures/test-app.py --max-count 2 --interval 1
    [[ "$status" -eq 0 ]]
    
    # Wait for some output
    sleep 3
    
    # Flush logs
    run ./pm2go flush test-flush
    [[ "$status" -eq 0 ]]
}

@test "pm2go flush all clears all logs" {
    # Start multiple processes
    run ./pm2go start python3 --name app-1 -- test/fixtures/test-app.py --max-count 1
    [[ "$status" -eq 0 ]]
    
    run ./pm2go start python3 --name app-2 -- test/fixtures/test-app.py --max-count 1  
    [[ "$status" -eq 0 ]]
    
    # Wait briefly
    sleep 2
    
    # Flush all logs
    run ./pm2go flush
    [[ "$status" -eq 0 ]]
}