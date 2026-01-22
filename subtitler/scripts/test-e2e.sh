#!/bin/bash
# Run end-to-end tests with Playwright

set -e

cd "$(dirname "$0")/../frontend"

echo "=== Running E2E Tests ==="
npm run test:e2e

echo ""
echo "=== E2E tests passed ==="
