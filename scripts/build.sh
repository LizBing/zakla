#!/bin/bash

# Build script for go-mc-server

set -e

echo "Building go-mc-server..."

# Get the script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# Create bin directory
mkdir -p bin

# Build server
echo "Building server..."
go build -o bin/server ./cmd/server

# Build mock client
echo "Building mock client..."
go build -o bin/mock-client ./cmd/mock-client

# Build real client
echo "Building real client..."
go build -o bin/real-client ./cmd/real-client

echo "Build complete! Binaries available in bin/"
