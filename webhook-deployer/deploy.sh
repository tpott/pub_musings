#!/bin/bash
# Deploy script for the personal website
# Called by webhook-deployer after git pull

set -e

SITE_PATH="${SITE_PATH:-/home/trevor/pub_musings/personal}"

cd "$SITE_PATH"

echo "Pulling latest changes..."
git pull origin trunk

echo "Loading nvm..."
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"

echo "Using Node version from .nvmrc..."
nvm use

echo "Installing dependencies..."
npm ci

echo "Building site..."
npm run build

echo "Deploy complete!"
