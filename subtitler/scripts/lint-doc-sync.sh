#!/bin/bash
# Check that documentation stays in sync with code.
# Catches common drift: undocumented env vars, routes, and dependencies.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BACKEND="$PROJECT_ROOT/backend"
FRONTEND="$PROJECT_ROOT/frontend"
DOCS="$PROJECT_ROOT/docs"

errors=0
warnings=0

warn() {
    echo "  WARNING: $1"
    warnings=$((warnings + 1))
}

err() {
    echo "  ERROR: $1"
    errors=$((errors + 1))
}

# --- Check 1: deps.md covers all go.mod direct dependencies ---
echo "Checking deps.md vs go.mod..."

# Extract direct (non-indirect) deps from go.mod
go_direct_deps=$(sed -n '/^require (/,/^)/p' "$BACKEND/go.mod" \
    | grep -v '// indirect' \
    | grep -oP '^\t\K[^\s]+' \
    | sort)

for dep in $go_direct_deps; do
    if ! grep -qF "$dep" "$DOCS/deps.md"; then
        err "Go dependency '$dep' not found in docs/deps.md"
    fi
done

# --- Check 2: deps.md covers all package.json deps ---
echo "Checking deps.md vs package.json..."

npm_deps=$(cd "$FRONTEND" && node -e "
const p=require('./package.json');
const all = Object.keys(p.dependencies||{}).concat(Object.keys(p.devDependencies||{}));
all.forEach(d => console.log(d));
" | sort)

for dep in $npm_deps; do
    if ! grep -qF "$dep" "$DOCS/deps.md"; then
        err "npm dependency '$dep' not found in docs/deps.md"
    fi
done

# --- Check 3: ENV.md covers all env vars used in Go code ---
echo "Checking ENV.md vs Go code env var usage..."

# Collect all env var names from non-test Go source files
# Use find to handle nested directories properly
env_vars=$(find "$BACKEND" -name '*.go' ! -name '*_test.go' -exec grep -hoP 'os\.Getenv\("\K[^"]+' {} + 2>/dev/null | sort -u)

# Also collect from getEnv* helper calls in config.go
config_env_vars=$(grep -oP 'getEnv\w+\("\K[^"]+' "$BACKEND/config.go" 2>/dev/null | sort -u)

all_env_vars=$(echo -e "$env_vars\n$config_env_vars" | sort -u)

for var in $all_env_vars; do
    if ! grep -qF "$var" "$DOCS/ENV.md"; then
        err "Environment variable '$var' not documented in docs/ENV.md"
    fi
done

# --- Check 4: API.md covers all registered routes ---
echo "Checking API.md vs registered routes..."

# Extract routes from HandleFunc calls (-h suppresses filename prefix)
routes=$(grep -hoP 'HandleFunc\("\K[^"]+' "$BACKEND"/handlers_*.go "$BACKEND"/main.go 2>/dev/null | sort -u)

while IFS= read -r route; do
    [ -z "$route" ] && continue
    # Extract path (second word after method)
    path=$(echo "$route" | cut -d' ' -f2)

    if ! grep -qF "$path" "$DOCS/API.md"; then
        warn "Route '$route' not found in docs/API.md"
    fi
done <<< "$routes"

# --- Check 5: RATE_LIMITS.md covers all rate limit env vars ---
echo "Checking RATE_LIMITS.md vs rate limit configuration..."

# Extract rate limit env var names from config.go
rate_limit_vars=$(grep -oP 'getEnvRateLimitOrDefault\("\K[^"]+' "$BACKEND/config.go" 2>/dev/null | sort -u)

for var in $rate_limit_vars; do
    if ! grep -qF "$var" "$DOCS/RATE_LIMITS.md"; then
        warn "Rate limit env var '$var' not documented in docs/RATE_LIMITS.md"
    fi
done

# --- Summary ---
echo ""
if [ $errors -gt 0 ]; then
    echo "Doc sync lint FAILED: $errors error(s), $warnings warning(s)"
    exit 1
elif [ $warnings -gt 0 ]; then
    echo "Doc sync lint passed with $warnings warning(s)"
else
    echo "Doc sync lint passed — all docs in sync with code"
fi
