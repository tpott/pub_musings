#!/bin/bash
# Run frontend unit tests

set -e

cd "$(dirname "$0")/../frontend"

echo "=== Running Frontend Tests ==="
npm test

echo ""
echo "=== Frontend tests passed ==="
