# PM2go E2E Tests

Simple end-to-end tests using Docker with systemd.

## Quick Start

```bash
# Build and run E2E tests
make test-e2e

# Or manually:
go build -o pm2go .
docker build -t pm2go-test -f test/Dockerfile .
cd test/e2e && go test -v -timeout 10m .
```

## What it tests

- Starting/stopping applications
- Environment variables (CLI and ecosystem files)
- Log viewing with `pm2go logs`
- JSON output with `pm2go jlist`
- Crash recovery and restarts
- Startup configuration

## Test Environment

- Ubuntu 22.04 with systemd
- `ubuntu` user with lingering enabled
- Test Node.js and Python applications
- All PM2go functionality working exactly like on real Linux

## Manual Testing

```bash
# Start container for manual testing
docker run -d --name pm2go-test --privileged --cgroupns=host \
  -v /sys/fs/cgroup:/sys/fs/cgroup:rw --tmpfs /run --tmpfs /tmp pm2go-test

# Copy binary and test manually
docker cp pm2go pm2go-test:/usr/local/bin/
docker exec -it pm2go-test sudo -u ubuntu bash
```

That's it! Simple and focused. 🎯