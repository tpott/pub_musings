#!/bin/bash
set -euo pipefail

cd /home/trevor/pub_musings
git fetch origin

# TODO run this in webhook-deployer's go code. leverage the config.yaml branch field
git checkout trunk
git pull origin trunk

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
