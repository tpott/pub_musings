#!/bin/bash
set -euo pipefail

cd /home/trevor/pub_musings
git fetch origin

# Checkout peek1 branch (peekaboo development branch)
git checkout peek1
git pull origin peek1

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
