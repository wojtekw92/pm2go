# PM2go Makefile

.PHONY: build test-vm clean

# Build pm2go binary
build:
	go build -o pm2go .

# Run E2E tests in VM (requires Linux environment)
test-vm:
	@echo "Running VM-based E2E tests..."
	@echo "Note: This must be run inside a Linux VM with systemd"
	./test/vm-test.sh

# Clean build artifacts
clean:
	rm -f pm2go