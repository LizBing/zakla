#!/bin/bash

#############################################
# Automated Debug Loop Script
#
# This script runs repeated iterations of:
# 1. Start the Minecraft server
# 2. Wait for server readiness
# 3. Run mock-client and capture logs
# 4. Run real-client and capture logs
# 5. Stop server and collect logs
# 6. Analyze results
# 7. Exit on success or continue to next iteration
#
# Usage: ./scripts/auto-debug.sh [max_iterations]
#############################################

set -e

# ===========================================
# Configuration
# ===========================================

# Maximum number of debug iterations (default: 100)
MAX_ITERATIONS=${1:-100}

# Project root directory
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Log directories
LOG_DIR="${PROJECT_ROOT}/logs"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
RUN_LOG_DIR="${LOG_DIR}/run_${TIMESTAMP}"

# Server and client paths (adjust as needed)
SERVER_DIR="${PROJECT_ROOT}/cmd/server"
MOCK_CLIENT_DIR="${PROJECT_ROOT}/cmd/mock-client"
REAL_CLIENT_DIR="${PROJECT_ROOT}/cmd/real-client"

# Analysis script
ANALYZE_SCRIPT="${PROJECT_ROOT}/scripts/analyze.js"

# ===========================================
# Color Output Functions
# ===========================================

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Print functions
print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

# ===========================================
# Cleanup and Signal Handling
# ===========================================

# Track background processes for cleanup
SERVER_PID=""
MOCK_PID=""
REAL_PID=""

cleanup() {
    print_info "Cleaning up processes..."

    # Stop docker-compose if running
    if docker-compose ps | grep -q "Up"; then
        print_info "Stopping docker-compose services..."
        docker-compose down 2>/dev/null || true
    fi

    # Kill any tracked background processes
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill "$SERVER_PID" 2>/dev/null || true
    fi
    if [ -n "$MOCK_PID" ] && kill -0 "$MOCK_PID" 2>/dev/null; then
        kill "$MOCK_PID" 2>/dev/null || true
    fi
    if [ -n "$REAL_PID" ] && kill -0 "$REAL_PID" 2>/dev/null; then
        kill "$REAL_PID" 2>/dev/null || true
    fi

    print_info "Cleanup complete"
}

# Set up signal handlers
trap cleanup SIGINT SIGTERM EXIT

# ===========================================
# Initialization
# ===========================================

init() {
    print_info "Initializing debug loop..."
    print_info "Max iterations: ${MAX_ITERATIONS}"
    print_info "Log directory: ${RUN_LOG_DIR}"

    # Create log directories
    mkdir -p "${RUN_LOG_DIR}"
    mkdir -p "${RUN_LOG_DIR}/server"
    mkdir -p "${RUN_LOG_DIR}/mock"
    mkdir -p "${RUN_LOG_DIR}/real"

    print_success "Initialization complete"
}

# ===========================================
# Server Functions
# ===========================================

start_server() {
    local iteration=$1

    print_info "Starting server (iteration ${iteration})..."

    # Start server using docker-compose
    # Adjust this command based on your setup
    cd "${PROJECT_ROOT}"

    # For docker-compose setup
    if [ -f "docker-compose.yml" ]; then
        docker-compose up -d
    else
        # Fallback: run server binary directly
        cd "${SERVER_DIR}" && go run . &
        SERVER_PID=$!
    fi

    print_success "Server started"
}

wait_for_server() {
    local iteration=$1
    local max_wait=60
    local waited=0

    print_info "Waiting for server to be ready..."

    while [ $waited -lt $max_wait ]; do
        # Check if server is accepting connections
        # Adjust this check based on your server
        if nc -z localhost 25565 2>/dev/null; then
            print_success "Server is ready"
            return 0
        fi

        sleep 2
        waited=$((waited + 2))
    done

    print_error "Server failed to become ready within ${max_wait} seconds"
    return 1
}

stop_server() {
    local iteration=$1

    print_info "Stopping server..."

    cd "${PROJECT_ROOT}"

    # Collect logs before stopping
    if [ -f "docker-compose.yml" ]; then
        docker-compose logs > "${RUN_LOG_DIR}/server/iteration_${iteration}.log"
        docker-compose down
    else
        if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
            kill "$SERVER_PID" 2>/dev/null || true
            wait "$SERVER_PID" 2>/dev/null || true
        fi
    fi

    print_success "Server stopped and logs collected"
}

# ===========================================
# Client Functions
# ===========================================

run_mock_client() {
    local iteration=$1

    print_info "Running mock-client..."

    local log_file="${RUN_LOG_DIR}/mock/iteration_${iteration}.log"

    # Run mock client and capture output
    cd "${MOCK_CLIENT_DIR}"

    if go run . > "${log_file}" 2>&1; then
        print_success "Mock client completed successfully"
        return 0
    else
        local exit_code=$?
        print_error "Mock client failed with exit code ${exit_code}"
        return 1
    fi
}

run_real_client() {
    local iteration=$1

    print_info "Running real-client..."

    local log_file="${RUN_LOG_DIR}/real/iteration_${iteration}.log"

    # Run real client and capture output
    cd "${REAL_CLIENT_DIR}"

    if go run . > "${log_file}" 2>&1; then
        print_success "Real client completed successfully"
        return 0
    else
        local exit_code=$?
        print_error "Real client failed with exit code ${exit_code}"
        return 1
    fi
}

# ===========================================
# Analysis Functions
# ===========================================

analyze_iteration() {
    local iteration=$1

    print_info "Analyzing results from iteration ${iteration}..."

    local server_log="${RUN_LOG_DIR}/server/iteration_${iteration}.log"
    local mock_log="${RUN_LOG_DIR}/mock/iteration_${iteration}.log"
    local real_log="${RUN_LOG_DIR}/real/iteration_${iteration}.log"

    # Call the analysis script
    if [ -x "$(command -v node)" ] && [ -f "${ANALYZE_SCRIPT}" ]; then
        local result
        result=$(node "${ANALYZE_SCRIPT}" "${server_log}" "${mock_log}" "${real_log}")
        echo "$result"
        return 0
    else
        print_warning "Analysis script not available, skipping analysis"
        echo "SKIP"
        return 0
    fi
}

# ===========================================
# Main Loop
# ===========================================

main() {
    init

    print_info "Starting debug loop..."
    echo ""

    for iteration in $(seq 1 "$MAX_ITERATIONS"); do
        print_info "========================================="
        print_info "Iteration ${iteration}/${MAX_ITERATIONS}"
        print_info "========================================="

        # Start server
        if ! start_server "$iteration"; then
            print_error "Failed to start server"
            continue
        fi

        # Wait for server readiness
        if ! wait_for_server "$iteration"; then
            print_error "Server not ready, stopping and continuing..."
            stop_server "$iteration"
            continue
        fi

        # Run mock client
        if ! run_mock_client "$iteration"; then
            print_error "Mock client failed"
        fi

        # Run real client
        if ! run_real_client "$iteration"; then
            print_error "Real client failed"
        fi

        # Stop server and collect logs
        stop_server "$iteration"

        # Analyze results
        local analysis_result
        analysis_result=$(analyze_iteration "$iteration")

        case "$analysis_result" in
            "SUCCESS")
                print_success "Analysis: SUCCESS!"
                print_info "Debug loop completed successfully after ${iteration} iterations"
                exit 0
                ;;
            "FAILURE")
                print_error "Analysis: FAILURE"
                print_info "Continuing to next iteration..."
                ;;
            *)
                print_warning "Analysis result: ${analysis_result}"
                print_info "Continuing to next iteration..."
                ;;
        esac

        echo ""
        sleep 1  # Brief pause between iterations
    done

    print_error "Debug loop completed without success after ${MAX_ITERATIONS} iterations"
    print_info "Logs available at: ${RUN_LOG_DIR}"
    exit 1
}

# Run main function
main "$@"
