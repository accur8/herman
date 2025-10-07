#!/usr/bin/env bash
set -e

# Test script for Herman

echo "=== Herman Test Setup ==="
echo

# Check if herman binary exists
if [ ! -f "../../herman" ]; then
    echo "Error: herman binary not found. Run 'go build -o herman ./src' first."
    exit 1
fi

# Get the absolute path to herman
HERMAN_PATH=$(cd ../.. && pwd)/herman
echo "Herman binary: $HERMAN_PATH"
echo

# Create a test directory
TEST_DIR=$(mktemp -d -t herman-test-XXXXX)
echo "Test directory: $TEST_DIR"
echo

# Copy the test config
cp a8-codegen.json "$TEST_DIR/a8-codegen.json"
echo "✓ Copied a8-codegen.json"

# Create symlink to herman
ln -s "$HERMAN_PATH" "$TEST_DIR/a8-codegen"
echo "✓ Created symlink: $TEST_DIR/a8-codegen -> $HERMAN_PATH"
echo

# Check if repo.properties exists
if [ ! -f "$HOME/.a8/repo.properties" ]; then
    echo "⚠️  Warning: $HOME/.a8/repo.properties not found"
    echo "   You need to create this file with your repo credentials."
    echo "   See repo.properties.example for an example."
    echo
    echo "   To create it:"
    echo "   mkdir -p ~/.a8"
    echo "   cp repo.properties.example ~/.a8/repo.properties"
    echo "   # Edit ~/.a8/repo.properties with your credentials"
    echo
else
    echo "✓ Found $HOME/.a8/repo.properties"
    echo
fi

echo "=== Test Commands ==="
echo
echo "To test herman, run:"
echo "  cd $TEST_DIR"
echo "  ./a8-codegen --help"
echo
echo "This will:"
echo "  1. Read a8-codegen.json"
echo "  2. Read ~/.a8/repo.properties"
echo "  3. Call the API to get nix build description"
echo "  4. Run nix-build to install"
echo "  5. Cache the result in ~/.a8/herman/builds/"
echo "  6. Execute the installed binary"
echo
echo "To clean up after testing:"
echo "  rm -rf $TEST_DIR"
echo "  rm -rf ~/.a8/herman/builds/io.accur8/a8-versions_3"
echo
