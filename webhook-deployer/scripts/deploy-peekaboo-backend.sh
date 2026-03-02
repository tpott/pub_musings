#!/bin/bash
set -euo pipefail

# Backend port - should match PORT in peekaboo.service
BACKEND_PORT="${BACKEND_PORT:-8070}"

cd /home/trevor/pub_musings
git fetch origin

# Checkout peek1 branch (peekaboo development branch)
git checkout peek1
git pull origin peek1

cd peekaboo

# Decrypt secrets to .env file
sops -d secrets.enc.yaml | grep -E '^[A-Z_]+:' | sed 's/: /=/' > .env

# Build from backend directory (where go.mod lives)
cd backend

# Build to temp file first (atomic swap)
CGO_ENABLED=1 go build -o ../peekaboo-new

cd ..

# Backup current binary
cp peekaboo peekaboo-prev 2>/dev/null || true

# Atomic move
mv peekaboo-new peekaboo

# Restart service
systemctl --user restart peekaboo

# Health check with retries
echo "Waiting for backend to start..."
for i in 1 2 3 4 5; do
    sleep 2
    if curl -sf --max-time 5 "http://localhost:${BACKEND_PORT}/health" > /dev/null; then
        echo "Peekaboo backend deployed successfully"
        exit 0
    fi
    echo "Attempt $i/5 failed, retrying..."
done

echo "WARNING: Health check failed after 5 attempts!"
curl --max-time 5 "http://localhost:${BACKEND_PORT}/health" 2>/dev/null || echo "Server not responding"

# Rollback to previous binary if available
if [[ -f peekaboo-prev ]]; then
    echo "Rolling back to previous binary..."
    mv peekaboo-prev peekaboo
    systemctl --user restart peekaboo
    sleep 2
    if curl -sf --max-time 5 "http://localhost:${BACKEND_PORT}/health" > /dev/null; then
        echo "Rollback succeeded — previous version is running"
    else
        echo "ERROR: Rollback also failed — manual intervention required"
    fi
else
    echo "No previous binary available for rollback"
fi
exit 1
