#!/bin/bash

# Debug loop script - rebuilds and reruns the server on file changes

set -e

echo "Starting debug loop..."
echo "Press Ctrl+C to exit"

# Get the script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# Check if fswatch is installed
if ! command -v fswatch &> /dev/null; then
    echo "fswatch is not installed. Install with:"
    echo "  brew install fswatch  # macOS"
    echo "  apt-get install fswatch  # Ubuntu/Debian"
    echo "Falling back to manual rebuild..."
fi

# Initial build
echo "Building..."
go build -o bin/server ./cmd/server

# Run server in background
echo "Starting server..."
./bin/server &
SERVER_PID=$!

# Trap to kill server on exit
trap "kill $SERVER_PID 2>/dev/null || true; exit" INT TERM EXIT

# Watch for changes and rebuild
if command -v fswatch &> /dev/null; then
    fswatch -o . | while read; do
        echo "Files changed, rebuilding..."
        kill $SERVER_PID 2>/dev/null || true

        go build -o bin/server ./cmd/server
        echo "Restarting server..."
        ./bin/server &
        SERVER_PID=$!
    done
else
    # Simple loop without file watching
    wait $SERVER_PID
fi
