#!/bin/bash
# Run linting for peekaboo backend and frontend
#
# Usage: ./scripts/lint.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== Linting Backend ==="
cd "$PROJECT_ROOT/backend"
gofmt -w .
go vet ./...
if command -v golangci-lint &> /dev/null; then
    golangci-lint run ./...
    echo "Backend linting passed (gofmt + go vet + golangci-lint)"
else
    echo "Backend linting passed (gofmt + go vet; install golangci-lint for full linting)"
fi

echo ""
echo "=== Building Frontend (includes type checking) ==="
cd "$PROJECT_ROOT/frontend"
npm run build
echo "Frontend build passed"

echo ""
echo "=== Checking Source File Sizes ==="
python3 "$SCRIPT_DIR/lint-filesize.py"

echo ""
echo "=== All linting passed ==="
