#!/bin/bash
# Run all verification: linting and tests
# This is the main script that should be run before commits

set -e

SCRIPT_DIR="$(dirname "$0")"

echo "========================================"
echo "Running Full Verification Suite"
echo "========================================"
echo ""

"$SCRIPT_DIR/lint.sh"

echo ""
echo "========================================"

"$SCRIPT_DIR/test-backend.sh"

echo ""
echo "========================================"

"$SCRIPT_DIR/test-frontend.sh"

echo ""
echo "========================================"
echo "ALL VERIFICATION PASSED"
echo "========================================"
