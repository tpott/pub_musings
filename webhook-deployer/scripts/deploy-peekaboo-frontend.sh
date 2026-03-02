#!/bin/bash
set -euo pipefail

cd /home/trevor/pub_musings
git fetch origin

# Checkout trunk branch (peekaboo development branch)
git checkout trunk
git pull origin trunk

cd peekaboo/frontend

# nvm has unbound variables internally, temporarily disable -u
set +u
. ~/.nvm/nvm.sh
nvm use
set -u

npm ci
npm run build

# Caddy serves directly from dist/ - no rsync needed

echo "Peekaboo frontend deployed successfully"
