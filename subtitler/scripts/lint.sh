#!/bin/bash
# Run linting for backend and frontend

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== Linting Backend ==="
cd "$PROJECT_ROOT/backend"
/home/trevor/go/bin/go fmt ./...
/home/trevor/go/bin/go vet ./...
echo "Backend linting passed"

echo ""
echo "=== Building Frontend (includes type checking) ==="
cd "$PROJECT_ROOT/frontend"
npm run build
echo "Frontend build passed"

echo ""
echo "=== All linting passed ==="
