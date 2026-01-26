#!/bin/bash
set -e  # Exit on error

PROJECT_DIR="/home/trevor/pub_musings/subtitler"
FRONTEND_DIR="$PROJECT_DIR/frontend"
BACKEND_DIR="$PROJECT_DIR/backend"
BACKEND_BINARY="$BACKEND_DIR/subtitler-server"

echo "[$(date)] Starting deployment..."

# Pull latest code
cd "$PROJECT_DIR"
echo "Pulling latest code from trunk..."
git pull origin trunk

# Build frontend
echo "Building frontend..."
cd "$FRONTEND_DIR"
npm ci --production=false
npm run build

# Build backend
echo "Building backend..."
cd "$BACKEND_DIR"
/home/trevor/go/bin/go build -o "$BACKEND_BINARY" ./cmd/server

# Restart backend service
echo "Restarting backend service..."
sudo systemctl restart subtitler-backend

# Wait for service to be ready
sleep 2

# Verify service is running
if sudo systemctl is-active --quiet subtitler-backend; then
    echo "[$(date)] Deployment completed successfully"
    exit 0
else
    echo "[$(date)] ERROR: Backend service failed to start"
    sudo journalctl -u subtitler-backend -n 50
    exit 1
fi
