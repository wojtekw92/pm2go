#!/usr/bin/env python3

import os
import sys
import time
import signal
import json
from datetime import datetime

class TestApp:
    def __init__(self):
        self.running = True
        self.counter = 0
        
    def signal_handler(self, signum, frame):
        print(f"Received signal {signum}, shutting down gracefully...")
        self.running = False

    def run(self):
        print("Test Python app starting...")
        print(f"Environment: {os.environ.get('PYTHON_ENV', 'development')}")
        print(f"Port: {os.environ.get('PORT', '8000')}")
        print(f"PID: {os.getpid()}")
        
        # Register signal handlers
        signal.signal(signal.SIGTERM, self.signal_handler)
        signal.signal(signal.SIGINT, self.signal_handler)
        
        # Print environment variables
        print("Environment variables:")
        for key, value in os.environ.items():
            if key.startswith('TEST_') or key.startswith('PYTHON_'):
                print(f"  {key}={value}")
        
        # Check for crash scenario
        crash_after = os.environ.get('CRASH_AFTER')
        crash_time = None
        if crash_after:
            crash_time = time.time() + int(crash_after)
            print(f"Will crash after {crash_after} seconds for testing")
        
        print("Test Python app is running. Use Ctrl+C to stop.")
        
        # Main loop
        while self.running:
            self.counter += 1
            timestamp = datetime.now().isoformat()
            print(f"[{timestamp}] Heartbeat #{self.counter} - PID: {os.getpid()}")
            
            # Test environment variables
            test_var = os.environ.get('TEST_VAR')
            if test_var:
                print(f"TEST_VAR: {test_var}")
            
            # Simulate memory usage reporting
            try:
                import psutil
                process = psutil.Process()
                mem_info = process.memory_info()
                print(f"Memory: RSS={mem_info.rss // 1024 // 1024}MB, VMS={mem_info.vms // 1024 // 1024}MB")
            except ImportError:
                print("Memory: psutil not available")
            
            # Check for crash condition
            if crash_time and time.time() >= crash_time:
                print("Simulating crash for testing...")
                sys.exit(1)
            
            time.sleep(5)
        
        print("Test Python app shutting down gracefully.")

if __name__ == "__main__":
    app = TestApp()
    try:
        app.run()
    except KeyboardInterrupt:
        print("Interrupted by user")
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)