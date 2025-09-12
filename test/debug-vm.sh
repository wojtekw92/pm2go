#!/bin/bash

# Debug script for VM testing issues

set -e

echo "PM2go VM Debug Script"
echo "===================="

# Check environment
echo "1. Environment checks:"
echo "   OS: $(uname -a)"
echo "   User: $(whoami)"
echo "   UID: $(id -u)"
echo "   Groups: $(groups)"

# Check systemd
echo ""
echo "2. Systemd checks:"
if command -v systemctl &> /dev/null; then
    echo "   ✓ systemctl available"
    
    echo "   System systemd status:"
    systemctl status --no-pager || echo "   ⚠ System systemd issues"
    
    echo "   User systemd status:"
    systemctl --user status --no-pager || echo "   ⚠ User systemd issues"
    
    echo "   XDG_RUNTIME_DIR: ${XDG_RUNTIME_DIR:-not set}"
    if [[ -n "$XDG_RUNTIME_DIR" ]]; then
        echo "   XDG_RUNTIME_DIR exists: $(ls -la $XDG_RUNTIME_DIR 2>/dev/null | wc -l) files"
    fi
else
    echo "   ✗ systemctl not available"
    exit 1
fi

# Check project structure
echo ""
echo "3. Project structure:"
if [[ -f "go.mod" ]]; then
    echo "   ✓ go.mod found"
    echo "   Module: $(grep module go.mod)"
else
    echo "   ✗ go.mod not found"
    exit 1
fi

if [[ -d "test/fixtures" ]]; then
    echo "   ✓ test fixtures found:"
    ls -la test/fixtures/
else
    echo "   ✗ test fixtures not found"
    exit 1
fi

# Try building pm2go
echo ""
echo "4. Build test:"
if go build -o pm2go-test .; then
    echo "   ✓ Build successful"
    rm pm2go-test
else
    echo "   ✗ Build failed"
    exit 1
fi

# Test basic systemd user service
echo ""
echo "5. Basic systemd test:"
cat > test-service.service << EOF
[Unit]
Description=Test Service

[Service]
ExecStart=/bin/sleep 30
Restart=always

[Install]
WantedBy=default.target
EOF

echo "   Creating test service..."
mkdir -p ~/.config/systemd/user
cp test-service.service ~/.config/systemd/user/
systemctl --user daemon-reload

echo "   Starting test service..."
if systemctl --user start test-service; then
    echo "   ✓ Test service started"
    systemctl --user status test-service --no-pager
    systemctl --user stop test-service
    systemctl --user disable test-service
    rm ~/.config/systemd/user/test-service.service
    systemctl --user daemon-reload
    echo "   ✓ Test service cleaned up"
else
    echo "   ✗ Test service failed"
    rm -f ~/.config/systemd/user/test-service.service
    rm -f test-service.service
    exit 1
fi

rm -f test-service.service

echo ""
echo "All checks passed! VM environment looks good for testing."