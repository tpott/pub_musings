#!/bin/bash
# Run backend tests

set -e

# Ensure Go is in PATH (for non-interactive shells)
if ! command -v go &> /dev/null && [ -d "$HOME/go/bin" ]; then
    export PATH="$PATH:$HOME/go/bin"
fi

cd "$(dirname "$0")/../backend"

echo "=== Running Backend Tests ==="
go test ./... -v

echo ""
echo "=== Backend tests passed ==="
