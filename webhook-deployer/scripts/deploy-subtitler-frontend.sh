#!/bin/bash
set -euo pipefail

cd /home/trevor/pub_musings
git fetch origin
git checkout subtitler_v3
git pull origin subtitler_v3

cd subtitler/frontend

# nvm has unbound variables internally, temporarily disable -u
set +u
. ~/.nvm/nvm.sh
nvm use
set -u

npm ci
npm run build

# Caddy serves directly from dist/ - no rsync needed

echo "Frontend deployed successfully"
