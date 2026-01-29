#!/bin/bash
# Run linting for backend and frontend

set -e

# Ensure Go is in PATH (for non-interactive shells)
if ! command -v go &> /dev/null && [ -d "$HOME/go/bin" ]; then
    export PATH="$PATH:$HOME/go/bin"
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== Linting Backend ==="
cd "$PROJECT_ROOT/backend"
go fmt ./...
go vet ./...
if command -v golangci-lint &> /dev/null; then
    golangci-lint run ./...
    echo "Backend linting passed (go fmt + go vet + golangci-lint)"
else
    echo "Backend linting passed (go fmt + go vet; install golangci-lint for full linting)"
fi

echo ""
echo "=== Building Frontend (includes type checking) ==="
cd "$PROJECT_ROOT/frontend"
npm run build
echo "Frontend build passed"

echo ""
echo "=== Checking Frontend File Sizes ==="
"$SCRIPT_DIR/lint-frontend-filesize.sh"

echo ""
echo "=== All linting passed ==="
