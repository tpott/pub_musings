#!/bin/bash
set -euo pipefail

cd /home/trevor/pub_musings
git fetch origin
git checkout subtitler_v3
git pull origin subtitler_v3

cd subtitler/backend

# Build to temp file first (atomic swap)
/home/trevor/go/bin/go build -o subtitler-new

# Backup current binary
sudo cp /opt/subtitler/backend/subtitler /opt/subtitler/backend/subtitler-prev 2>/dev/null || true

# Atomic move
sudo mv subtitler-new /opt/subtitler/backend/subtitler

# Restart service
sudo systemctl restart subtitler

# Health check
sleep 2
if curl -sf http://localhost:8080/api/health > /dev/null; then
    echo "Backend deployed successfully"
else
    echo "WARNING: Health check failed!"
    exit 1
fi
