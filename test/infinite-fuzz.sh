#!/usr/bin/env bash

set -u  # but NOT `set -e`, we expect failures

# Script name for tracking running instances
SCRIPT_NAME="$(basename "$0")"

# Function to kill all running fuzzing processes
kill_fuzzing() {
    echo "=========================================="
    echo "Stopping all fuzzing processes..."
    echo "=========================================="

    local killed=0

    # Kill the infinite-fuzz.sh script instances
    if pgrep -f "$SCRIPT_NAME" > /dev/null 2>&1; then
        echo "Killing $SCRIPT_NAME instances..."
        pkill -9 -f "$SCRIPT_NAME" 2>/dev/null || true
        killed=1
    fi

    # Kill all test.test processes (fuzzing workers)
    if pgrep -f "test.test.*fuzz" > /dev/null 2>&1; then
        echo "Killing fuzz test workers..."
        pkill -9 -f "test.test.*fuzz" 2>/dev/null || true
        killed=1
    fi

    # Kill any go test fuzz commands
    if pgrep -f "go test.*fuzz" > /dev/null 2>&1; then
        echo "Killing go test commands..."
        pkill -9 -f "go test.*fuzz" 2>/dev/null || true
        killed=1
    fi

    sleep 1

    # Verify cleanup
    if pgrep -f "fuzz" > /dev/null 2>&1; then
        echo ""
        echo "⚠️  Some processes may still be running:"
        ps aux | grep -E "(infinite-fuzz|fuzz|test.test)" | grep -v grep
        echo ""
        echo "If needed, manually kill with: pkill -9 -f fuzz"
    else
        if [ $killed -eq 1 ]; then
            echo "✅ All fuzzing processes stopped"
        else
            echo "No fuzzing processes found"
        fi
    fi

    exit 0
}

# Function to discover all Fuzz functions in the current directory
discover_fuzz_targets() {
    local targets=()

    # Find all *_test.go files and extract Fuzz function names
    while IFS= read -r line; do
        targets+=("$line")
    done < <(grep -h "^func Fuzz" ./*_test.go 2>/dev/null | sed 's/func \(Fuzz[^(]*\).*/\1/' | sort -u)

    # Print each target on a separate line
    printf '%s\n' "${targets[@]}"
}

# Usage function
usage() {
    cat <<EOF
Usage: $SCRIPT_NAME [OPTIONS]

Run Go fuzz tests continuously in parallel until stopped.

Options:
  -k, --kill       Kill all running fuzzing processes and exit
  -t, --targets    Comma-separated list of fuzz targets (default: auto-discover)
  -h, --help       Show this help message

Examples:
  $SCRIPT_NAME                    # Auto-discover and run all Fuzz* functions
  $SCRIPT_NAME -t FuzzFoo,FuzzBar # Run specific targets
  $SCRIPT_NAME -k                 # Kill all running fuzzing

Auto-discovery:
  This script automatically finds all functions matching 'func Fuzz*'
  in *_test.go files in the current directory.

Stopping:
  Press Ctrl+C or run: $SCRIPT_NAME -k

EOF
    exit 0
}

# Parse command line arguments
TARGETS=""
while [[ $# -gt 0 ]]; do
    case $1 in
        -k|--kill)
            kill_fuzzing
            ;;
        -t|--targets)
            TARGETS="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Array to track background PIDs
pids=()

# Cleanup function to kill all background processes
cleanup() {
    echo ""
    echo "=========================================="
    echo "Stopping fuzzing at $(date)"
    echo "Killing background processes..."
    echo "=========================================="

    # Kill all background processes
    for pid in "${pids[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            echo "Killing PID $pid"
            kill "$pid" 2>/dev/null || true
        fi
    done

    # Give them a moment to die gracefully
    sleep 1

    # Force kill any that remain
    for pid in "${pids[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            echo "Force killing PID $pid"
            kill -9 "$pid" 2>/dev/null || true
        fi
    done

    echo "All fuzzing processes stopped"
    exit 0
}

# Set up trap to catch Ctrl+C (SIGINT) and other termination signals
trap cleanup SIGINT SIGTERM

# Function to run a single fuzz target continuously
fuzz_target() {
    local target="$1"
    local run_count=0

    while true; do
        run_count=$((run_count + 1))
        echo "[${target}] Starting fuzz run #${run_count} at $(date)"

        # Run go test with GOEXPERIMENT (default to jsonv2 if not set)
        GOEXPERIMENT="${GOEXPERIMENT:-jsonv2}" go test -run=^$ -fuzz=^${target}$
        status=$?

        echo "[${target}] Fuzz run #${run_count} finished with status ${status} at $(date)"

        # If fuzzing found a crash (non-zero exit), log it
        if [ $status -ne 0 ]; then
            echo "[${target}] ⚠️  Found issue! Check testdata/fuzz/${target}/ for details"
        fi

        # Small sleep to avoid hammering CPU between runs
        sleep 1
    done
}

# Discover or parse targets
if [ -z "$TARGETS" ]; then
    # Auto-discover Fuzz functions
    echo "Auto-discovering fuzz targets..."
    FUZZ_TARGETS=()
    while IFS= read -r target; do
        FUZZ_TARGETS+=("$target")
    done < <(discover_fuzz_targets)

    if [ ${#FUZZ_TARGETS[@]} -eq 0 ]; then
        echo "Error: No Fuzz* functions found in *_test.go files"
        echo "Make sure you're in a directory with fuzz tests"
        exit 1
    fi

    echo "Found ${#FUZZ_TARGETS[@]} fuzz target(s): ${FUZZ_TARGETS[*]}"
else
    # Use provided targets
    IFS=',' read -ra FUZZ_TARGETS <<< "$TARGETS"
fi

echo "=========================================="
echo "Starting infinite fuzzing at $(date)"
echo "Targets: ${FUZZ_TARGETS[*]}"
echo "Press Ctrl+C to stop or run: $SCRIPT_NAME -k"
echo "=========================================="
echo ""

# Run all fuzz targets in parallel and track their PIDs
for target in "${FUZZ_TARGETS[@]}"; do
    fuzz_target "$target" &
    pids+=($!)
done

# Wait for all background processes (they run forever until Ctrl+C)
wait
