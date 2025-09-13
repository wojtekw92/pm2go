#!/usr/bin/env python3
"""
Simple test application for PM2go testing.
Produces output every 2 seconds with timestamps and optional arguments.
"""

import sys
import time
import argparse
import os
from datetime import datetime

def main():
    parser = argparse.ArgumentParser(description='PM2go test application')
    parser.add_argument('--interval', '-i', type=int, default=2, 
                       help='Output interval in seconds (default: 2)')
    parser.add_argument('--max-count', '-c', type=int, default=0,
                       help='Maximum number of outputs (0 = infinite)')
    parser.add_argument('--error-every', '-e', type=int, default=0,
                       help='Print to stderr every N iterations (0 = never)')
    parser.add_argument('--message', '-m', type=str, default='Hello from PM2go test app',
                       help='Custom message to output')
    parser.add_argument('--env-vars', action='store_true',
                       help='Print environment variables on startup')
    
    args = parser.parse_args()
    
    # Print startup info
    print(f"=== PM2go Test App Started ===")
    print(f"PID: {os.getpid()}")
    print(f"Args: {sys.argv}")
    print(f"Interval: {args.interval}s")
    print(f"Max count: {args.max_count if args.max_count > 0 else 'infinite'}")
    
    # Print environment variables if requested
    if args.env_vars:
        print("=== Environment Variables ===")
        for key, value in sorted(os.environ.items()):
            if key.startswith(('TEST_', 'PM2GO_', 'NODE_', 'PYTHON_')):
                print(f"{key}={value}")
        print("=============================")
    
    count = 0
    
    try:
        while True:
            count += 1
            timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
            
            # Regular output to stdout
            print(f"[{timestamp}] #{count}: {args.message}")
            sys.stdout.flush()
            
            # Optional error output
            if args.error_every > 0 and count % args.error_every == 0:
                print(f"[{timestamp}] ERROR #{count}: This is an error message", file=sys.stderr)
                sys.stderr.flush()
            
            # Check if we've reached max count
            if args.max_count > 0 and count >= args.max_count:
                print(f"[{timestamp}] Reached max count ({args.max_count}), exiting")
                break
                
            time.sleep(args.interval)
            
    except KeyboardInterrupt:
        print(f"[{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}] Received SIGINT, shutting down gracefully")
        print(f"Total outputs: {count}")
    except Exception as e:
        print(f"[{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}] ERROR: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()