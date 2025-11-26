#!/usr/bin/env bash

set -u  # but NOT `set -e`, we expect failures

while true; do
    echo "Starting fuzz run at $(date)"
    go test -run=^$ -fuzz=^FuzzParseSQL$
    status=$?
    echo "Fuzz run finished with status ${status} at $(date)"
    # Optional small sleep to avoid hammering CPU between runs
    sleep 1
done
