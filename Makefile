# PM2go Makefile

.PHONY: build test test-go test-vm clean

# Build pm2go binary
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/wojtekw92/pm2go/cmd.version=$(VERSION)

build:
	go build -ldflags '$(LDFLAGS)' -o pm2go .

# Run fast Go checks
test-go:
	go test ./...
	go test -race ./...
	go vet ./...

# Run full local test suite
test: test-go
	./test/run-tests.sh

# Run E2E tests in VM (requires Linux environment)
test-vm:
	@echo "Running VM-based E2E tests..."
	@echo "Note: This must be run inside a Linux VM with systemd"
	./test/run-tests.sh

# Clean build artifacts
clean:
	rm -f pm2go
