#!/bin/bash
# Run backend tests

set -e

cd "$(dirname "$0")/../backend"

echo "=== Running Backend Tests ==="
/home/trevor/go/bin/go test ./... -v

echo ""
echo "=== Backend tests passed ==="
