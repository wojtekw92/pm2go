#!/bin/bash
# Test validation script - checks test structure without running bats

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "=== PM2go Test Structure Validation ==="
echo

# Check if test files exist
test_files=(
    "test/basic.bats"
    "test/ids-and-bulk.bats" 
    "test/inspection.bats"
    "test/logging.bats"
    "test/ecosystem.bats"
    "test/environment.bats"
    "test/json-output.bats"
)

echo "✓ Checking test files..."
for file in "${test_files[@]}"; do
    if [[ -f "$file" ]]; then
        echo "  ✓ $file exists"
    else
        echo "  ✗ $file missing"
    fi
done

echo

# Check fixtures
echo "✓ Checking test fixtures..."
if [[ -f "test/fixtures/test-app.py" ]]; then
    echo "  ✓ test-app.py exists"
    if [[ -x "test/fixtures/test-app.py" ]]; then
        echo "  ✓ test-app.py is executable"
    else
        echo "  ⚠ test-app.py not executable"
    fi
else
    echo "  ✗ test-app.py missing"
fi

if [[ -f "test/fixtures/test-ecosystem.json" ]]; then
    echo "  ✓ test-ecosystem.json exists"
else
    echo "  ✗ test-ecosystem.json missing"
fi

echo

# Check test app functionality
echo "✓ Testing test application..."
if python3 test/fixtures/test-app.py --max-count 1 --interval 0.5 >/dev/null 2>&1; then
    echo "  ✓ test-app.py runs successfully"
else
    echo "  ✗ test-app.py failed to run"
fi

echo

# Count tests in each file
echo "✓ Counting tests in each file..."
for file in "${test_files[@]}"; do
    if [[ -f "$file" ]]; then
        count=$(grep -c '^@test' "$file")
        echo "  $file: $count tests"
    fi
done

echo

# Check for bats syntax issues
echo "✓ Checking basic bats syntax..."
syntax_ok=true
for file in "${test_files[@]}"; do
    if [[ -f "$file" ]]; then
        # Check for common syntax issues
        if ! grep -q '#!/usr/bin/env bats' "$file"; then
            echo "  ⚠ $file missing bats shebang"
            syntax_ok=false
        fi
        
        # Check for proper test structure
        if ! grep -q '^@test' "$file"; then
            echo "  ⚠ $file has no tests"
            syntax_ok=false
        fi
        
        # Check for setup/teardown functions
        if grep -q 'setup()' "$file" && grep -q 'teardown()' "$file"; then
            echo "  ✓ $file has setup/teardown functions"
        fi
    fi
done

if $syntax_ok; then
    echo "  ✓ Basic syntax checks passed"
fi

echo

# Check if pm2go can be built
echo "✓ Checking if PM2go builds..."
if go build -o pm2go >/dev/null 2>&1; then
    echo "  ✓ PM2go builds successfully"
else
    echo "  ✗ PM2go build failed"
fi

echo

# Summary
echo "=== Validation Summary ==="
total_tests=0
for file in "${test_files[@]}"; do
    if [[ -f "$file" ]]; then
        count=$(grep -c '^@test' "$file")
        total_tests=$((total_tests + count))
    fi
done

echo "📊 Total test files: ${#test_files[@]}"
echo "📊 Total tests: $total_tests"
echo "📊 Test fixtures: 2 (test-app.py, test-ecosystem.json)"

echo
echo "🚀 To run tests, install bats-core and run:"
echo "   ./test/run-tests.sh"
echo
echo "📖 See test/README.md for detailed instructions"