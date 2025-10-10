#!/usr/bin/env bash
set -euo pipefail

# Directory containing this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HERMAN="${HERMAN:-$REPO_ROOT/result/bin/herman}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track results
PASSED=0
FAILED=0
FAILED_TESTS=()

echo "Herman Test Suite"
echo "================="
echo "Using herman: $HERMAN"
echo ""

# Check if herman binary exists
if [ ! -x "$HERMAN" ]; then
    echo -e "${RED}Error: Herman binary not found at $HERMAN${NC}"
    echo "Please build herman first with: nix build"
    exit 1
fi

# Find all testcase directories
for testcase_dir in "$SCRIPT_DIR"/testcase-*/; do
    if [ ! -d "$testcase_dir" ]; then
        continue
    fi

    testcase_name=$(basename "$testcase_dir")

    # Check if required files exist
    if [ ! -f "$testcase_dir/app.json" ]; then
        echo -e "${YELLOW}⊘ Skipping $testcase_name: missing app.json${NC}"
        continue
    fi
    if [ ! -f "$testcase_dir/dependencies.json" ]; then
        echo -e "${YELLOW}⊘ Skipping $testcase_name: missing dependencies.json${NC}"
        continue
    fi
    if [ ! -f "$testcase_dir/expected-default.nix" ]; then
        echo -e "${YELLOW}⊘ Skipping $testcase_name: missing expected-default.nix${NC}"
        continue
    fi

    echo -n "Testing $testcase_name... "

    # Create temp directory for output
    output_dir=$(mktemp -d)
    trap "rm -rf $output_dir" EXIT

    # Run herman generate
    if ! "$HERMAN" generate \
        "$testcase_dir/app.json" \
        "$output_dir" \
        --dependencies-json "$testcase_dir/dependencies.json" \
        > "$output_dir/stdout.txt" 2> "$output_dir/stderr.txt"; then
        echo -e "${RED}✗ FAILED (herman command failed)${NC}"
        echo "  Error output:"
        cat "$output_dir/stderr.txt" | sed 's/^/    /'
        FAILED=$((FAILED + 1))
        FAILED_TESTS+=("$testcase_name (command failed)")
        continue
    fi

    # Compare default.nix
    if ! diff -u "$testcase_dir/expected-default.nix" "$output_dir/default.nix" > "$output_dir/default.diff" 2>&1; then
        echo -e "${RED}✗ FAILED (default.nix mismatch)${NC}"
        echo "  Diff:"
        cat "$output_dir/default.diff" | head -20 | sed 's/^/    /'
        if [ $(wc -l < "$output_dir/default.diff") -gt 20 ]; then
            echo "    ... ($(wc -l < "$output_dir/default.diff") lines total, showing first 20)"
        fi
        FAILED=$((FAILED + 1))
        FAILED_TESTS+=("$testcase_name (default.nix)")
        continue
    fi

    # Compare flake.nix if expected file exists
    if [ -f "$testcase_dir/expected-flake.nix" ]; then
        if ! diff -u "$testcase_dir/expected-flake.nix" "$output_dir/flake.nix" > "$output_dir/flake.diff" 2>&1; then
            echo -e "${RED}✗ FAILED (flake.nix mismatch)${NC}"
            echo "  Diff:"
            cat "$output_dir/flake.diff" | head -20 | sed 's/^/    /'
            if [ $(wc -l < "$output_dir/flake.diff") -gt 20 ]; then
                echo "    ... ($(wc -l < "$output_dir/flake.diff") lines total, showing first 20)"
            fi
            FAILED=$((FAILED + 1))
            FAILED_TESTS+=("$testcase_name (flake.nix)")
            continue
        fi
    fi

    echo -e "${GREEN}✓ PASSED${NC}"
    PASSED=$((PASSED + 1))
done

# Summary
echo ""
echo "================="
echo "Test Results:"
echo "  Passed: $PASSED"
echo "  Failed: $FAILED"

if [ $FAILED -gt 0 ]; then
    echo ""
    echo "Failed tests:"
    for test in "${FAILED_TESTS[@]}"; do
        echo "  - $test"
    done
    echo ""
    exit 1
fi

echo ""
echo -e "${GREEN}All tests passed!${NC}"
exit 0
