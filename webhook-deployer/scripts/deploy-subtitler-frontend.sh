#!/bin/bash
set -euo pipefail

cd /home/trevor/pub_musings
git fetch origin
git checkout subtitler_v3
git pull origin subtitler_v3

cd subtitler/frontend
. ~/.nvm/nvm.sh && nvm use
npm ci
npm run build

# Caddy serves directly from dist/ - no rsync needed

echo "Frontend deployed successfully"
