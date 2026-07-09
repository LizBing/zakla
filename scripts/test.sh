#!/bin/bash

# Test script for go-mc-server

set -e

echo "Running tests..."

# Get the script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# Run tests with coverage
echo "Running go tests..."
go test -v -race -coverprofile=coverage.out ./pkg/...

# Show coverage
go tool cover -func=coverage.out

echo "Tests complete!"
