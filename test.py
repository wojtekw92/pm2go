#!/usr/bin/env python3
import os
print("Environment variables:")
for key, value in sorted(os.environ.items()):
    print(f"{key}={value}")