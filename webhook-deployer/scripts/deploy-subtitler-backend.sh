#!/bin/bash
set -euo pipefail

# Backend port - should match PORT in subtitler.service
BACKEND_PORT="${BACKEND_PORT:-8060}"

cd /home/trevor/pub_musings
git fetch origin
git checkout subtitler_v3
git pull origin subtitler_v3

cd subtitler/backend

# Build to temp file first (atomic swap)
CGO_ENABLED=1 go build -o subtitler-new

# Backup current binary
cp subtitler subtitler-prev 2>/dev/null || true

# Atomic move
mv subtitler-new subtitler

# Restart service
systemctl --user restart subtitler

# Health check with retries
echo "Waiting for backend to start..."
for i in 1 2 3 4 5; do
    sleep 2
    if curl -sf http://localhost:${BACKEND_PORT}/api/health > /dev/null; then
        echo "Backend deployed successfully"
        exit 0
    fi
    echo "Attempt $i/5 failed, retrying..."
done

echo "WARNING: Health check failed after 5 attempts!"
curl http://localhost:${BACKEND_PORT}/api/health 2>/dev/null || echo "Server not responding"
exit 1
