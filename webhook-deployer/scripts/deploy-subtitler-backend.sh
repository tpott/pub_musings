#!/bin/bash
set -euo pipefail

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

# Health check
sleep 2
if curl -sf http://localhost:8080/api/health > /dev/null; then
    echo "Backend deployed successfully"
else
    echo "WARNING: Health check failed!"
    exit 1
fi
