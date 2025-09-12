# PM2go Makefile

.PHONY: build test test-e2e clean

# Build pm2go binary
build:
	go build -o pm2go .

# Run E2E tests  
test-e2e: build
	@echo "Building test container..."
	docker build -t pm2go-test -f test/Dockerfile .
	@echo "Running E2E tests..."
	cd test/e2e && go test -v -timeout 10m .
	@echo "Cleaning up..."
	docker stop pm2go-e2e-test 2>/dev/null || true
	docker rm pm2go-e2e-test 2>/dev/null || true

# Run tests (only E2E for now)
test: test-e2e

# Clean build artifacts
clean:
	rm -f pm2go
	docker rmi pm2go-test 2>/dev/null || true