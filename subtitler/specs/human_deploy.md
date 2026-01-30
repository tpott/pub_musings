# Human Deploy: Multi-Branch Support for webhook-deployer

## Goal
Modify webhook-deployer to support multiple branches, specifically adding support for the `subtitler_v3` branch to trigger frontend and backend deployments for the subtitler application.

## Architecture Context
- **webhook-deployer runs inside the Ubuntu VM** (not on Mac host)
- The VM has `~/pub_musings` checked out (the entire repo)
- Cloudflare tunnel routes `webhook.pottingers.us` → localhost:9000
- When a push webhook arrives, it builds locally on the VM - no SSH, no cross-compilation

## Current State
- Hardcoded `trunk` branch filtering at `webhook.go:71-77`
- Single `SITE_PATH` environment variable
- Inline bash deployment script

## Implementation

### Files to Create

#### 1. `config.go` - Configuration types and loader
```go
type Config struct {
    Sites []SiteConfig `yaml:"sites"`
}

type SiteConfig struct {
    Name         string            `yaml:"name"`
    Path         string            `yaml:"path"`          // Working directory
    PathPrefix   string            `yaml:"path_prefix"`   // Only deploy if files here changed
    Branch       string            `yaml:"branch"`
    Repository   string            `yaml:"repository"`    // e.g., "tpott/pub_musings"
    DeployScript string            `yaml:"deploy_script"` // External script
    Commands     []string          `yaml:"commands"`      // Or inline commands
    Environment  map[string]string `yaml:"environment"`
}
```

**Key addition: `path_prefix`** - Inspect GitHub payload's `commits[].added/modified/removed` arrays to only trigger deploy when files in that path changed.

#### 2. `config.yaml` - Deployment configuration
```yaml
sites:
  - name: personal
    path: /home/trevor/pub_musings/personal
    path_prefix: personal/
    branch: trunk
    repository: tpott/pub_musings
    commands:
      - "git pull origin trunk"
      - ". ~/.nvm/nvm.sh && nvm use"
      - "npm ci"
      - "npm run build"

  - name: subtitler-frontend
    path: /home/trevor/pub_musings/subtitler/frontend
    path_prefix: subtitler/frontend/
    branch: subtitler_v3
    repository: tpott/pub_musings
    deploy_script: /home/trevor/pub_musings/webhook-deployer/scripts/deploy-subtitler-frontend.sh

  - name: subtitler-backend
    path: /home/trevor/pub_musings/subtitler/backend
    path_prefix: subtitler/backend/
    branch: subtitler_v3
    repository: tpott/pub_musings
    deploy_script: /home/trevor/pub_musings/webhook-deployer/scripts/deploy-subtitler-backend.sh
```

**Benefit of separate targets:** If you only change frontend code, only frontend deploys. Backend stays untouched (no restart).

#### 3. `scripts/deploy-subtitler-frontend.sh`
```bash
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
```

#### 4. `scripts/deploy-subtitler-backend.sh`
```bash
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

# Restart user service (no sudo required)
systemctl --user restart subtitler

# Health check
sleep 2
if curl -sf http://localhost:8070/api/health > /dev/null; then
    echo "Backend deployed successfully"
else
    echo "WARNING: Health check failed!"
    exit 1
fi
```

### Files to Modify

#### 1. `webhook.go`
- Replace hardcoded `trunk` check with config-based site matching
- Add per-site mutex for sequential same-site deploys
- Add `extractBranch()`: `"refs/heads/subtitler_v3"` → `"subtitler_v3"`
- Add `findAffectedSites()` using extended GitHub payload
- Add `runDeployScript()` and `runInlineCommands()` methods

**Extended GitHub payload for path filtering:**
```go
type GitHubPushEvent struct {
    Ref        string `json:"ref"`
    Repository struct {
        FullName string `json:"full_name"`
    } `json:"repository"`
    Pusher struct {
        Name string `json:"name"`
    } `json:"pusher"`
    Commits []struct {
        Added    []string `json:"added"`
        Modified []string `json:"modified"`
        Removed  []string `json:"removed"`
    } `json:"commits"`
}
```

#### 2. `main.go`
- Add `CONFIG_PATH` env var (default: `./config.yaml`)
- Load config at startup
- Pass config to `NewWebhookHandler()` instead of `sitePath`

#### 3. `go.mod`
- Add: `gopkg.in/yaml.v3 v3.0.1`

### Files to Create for Documentation

#### 4. `NOTES.md` - Corrections to deployment spec
Document what's incorrect in `subtitler/specs/deployment.md`:
- Spec assumes building on Mac host with cross-compilation
- Spec assumes SSH from host to VM
- Reality: webhook-deployer runs inside VM, builds natively

## Implementation Order

1. Create `config.go` with types and loader
2. Create `config.yaml` with trunk config (preserves current behavior)
3. Update `go.mod` with yaml dependency
4. Modify `main.go` to load config
5. Modify `webhook.go` for multi-branch routing
6. Test trunk deployment still works
7. Create `scripts/deploy-subtitler.sh`
8. Add `subtitler_v3` to `config.yaml`
9. Create `NOTES.md` documenting spec corrections
10. Test subtitler deployment

## Critical Files

| File | Action |
|------|--------|
| `webhook-deployer/webhook.go` | Modify |
| `webhook-deployer/main.go` | Modify |
| `webhook-deployer/go.mod` | Modify |
| `webhook-deployer/config.go` | Create |
| `webhook-deployer/config.yaml` | Create |
| `webhook-deployer/scripts/deploy-subtitler-frontend.sh` | Create |
| `webhook-deployer/scripts/deploy-subtitler-backend.sh` | Create |
| `subtitler/NOTES.md` | Create |

## Verification

1. **Unit test**: Push to untracked branch → ignored with log message
2. **Integration test**: Push to `trunk` → existing deployment works
3. **E2E test**: Push to `subtitler_v3` → frontend + backend deployed

Manual verification on VM:
```bash
# Check frontend deployed
curl http://localhost:8060/  # Through Caddy

# Check backend running
systemctl --user status subtitler
curl http://localhost:8070/api/health
```

## VM Prerequisites

The backend runs as a **user systemd service** (no sudo required for deploys).

### One-time setup for user service:
```bash
# Create user systemd directory
mkdir -p ~/.config/systemd/user

# Create the service file at ~/.config/systemd/user/subtitler.service
# (see example below)

# Enable lingering so service starts at boot without login
loginctl enable-linger trevor

# Reload and enable
systemctl --user daemon-reload
systemctl --user enable subtitler
systemctl --user start subtitler
```

### Example `~/.config/systemd/user/subtitler.service`:
```ini
[Unit]
Description=Subtitler Backend
After=network.target

[Service]
Type=simple
WorkingDirectory=/home/trevor/pub_musings/subtitler/backend
ExecStart=/home/trevor/pub_musings/subtitler/backend/subtitler
Restart=always
RestartSec=5

# Environment (loaded from encrypted secrets via sops)
EnvironmentFile=/home/trevor/pub_musings/subtitler/backend/.env

# Security
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/home/trevor/pub_musings/subtitler/backend/data
ReadWritePaths=/home/trevor/pub_musings/subtitler/backend/uploads

[Install]
WantedBy=default.target
```

### Secrets Management with sops + age

Subtitler uses **sops + age** for encrypted secrets, following the same pattern as `personal/` and `webhook-deployer/`.

#### Required secrets for subtitler backend:
| Secret | Purpose |
|--------|---------|
| `PORT` | HTTP port (default: 8080) |
| `WHISPER_SERVER_URL` | URL of whisper-server (e.g., `http://10.0.2.2:8050`) |
| `WHISPER_MODEL` | Whisper model name (optional) |
| `USE_WHISPER_SERVER` | Set to `true` to use external whisper server |
| `RESEND_API_KEY` | Resend API key for email service |
| `EMAIL_FROM` | Sender email address (e.g., `noreply@subtitler.app`) |
| `EMAIL_ENABLED` | Set to `false` to disable email (defaults to enabled) |
| `APP_URL` | Application URL for email links (e.g., `https://subtitler.pottingers.us`) |
| `HTTPS_ONLY` | Set to `true` for secure cookies in production |

#### Add subtitler secrets to `pub_musings/secrets.enc.yaml`:

If you already have sops+age set up (see `personal/001_INITIALIZATION.md`), decrypt, add subtitler secrets, and re-encrypt:

```bash
cd ~/pub_musings

# Decrypt existing secrets
sops -d secrets.enc.yaml > secrets.yaml

# Add subtitler secrets to secrets.yaml:
# SUBTITLER_PORT: "8080"
# SUBTITLER_WHISPER_SERVER_URL: "http://10.0.2.2:8050"
# SUBTITLER_WHISPER_MODEL: "base"
# SUBTITLER_USE_WHISPER_SERVER: "true"
# SUBTITLER_RESEND_API_KEY: "re_xxxxx"
# SUBTITLER_EMAIL_FROM: "noreply@subtitler.pottingers.us"
# SUBTITLER_EMAIL_ENABLED: "true"
# SUBTITLER_APP_URL: "https://subtitler.pottingers.us"
# SUBTITLER_HTTPS_ONLY: "true"

# Re-encrypt
sops -e secrets.yaml > secrets.enc.yaml
rm secrets.yaml
```

#### Decrypt secrets for subtitler on the VM:

```bash
cd ~/pub_musings

# Extract subtitler secrets to .env file
sops -d secrets.enc.yaml | \
  grep -E "^SUBTITLER_" | \
  sed 's/^SUBTITLER_//' | \
  sed 's/: /=/' | sed 's/"//g' > subtitler/backend/.env

# Secure the file
chmod 600 subtitler/backend/.env

# Verify
cat subtitler/backend/.env
# Should show:
# PORT=8080
# WHISPER_SERVER_URL=http://10.0.2.2:8050
# WHISPER_MODEL=base
# USE_WHISPER_SERVER=true
# RESEND_API_KEY=re_xxxxx
# EMAIL_FROM=noreply@subtitler.pottingers.us
# EMAIL_ENABLED=true
# APP_URL=https://subtitler.pottingers.us
# HTTPS_ONLY=true
```

#### After updating secrets, reload the service:
```bash
systemctl --user daemon-reload
systemctl --user restart subtitler
```

### Cloudflare Tunnel DNS Setup

The VM already has a Cloudflare tunnel configured. To route `subtitler.pottingers.us` through the existing tunnel:

```bash
# List existing tunnels to find the tunnel name
cloudflared tunnel list

# Check which tunnel ID is configured in /etc/cloudflared/config.yml
cat /etc/cloudflared/config.yml | grep tunnel:
# Example output: tunnel: abc123-def456-...

# Match the tunnel ID to the name from `tunnel list`, then route DNS
# This creates a CNAME record in Cloudflare DNS automatically
cloudflared tunnel route dns <tunnel-name> subtitler.pottingers.us
```

Then update `/etc/cloudflared/config.yml` to add the subtitler ingress rule:

```yaml
tunnel: <tunnel-id>
credentials-file: /home/trevor/.cloudflared/<tunnel-id>.json

ingress:
  - hostname: t.pottingers.us
    service: http://localhost:8080
  - hostname: webhook.pottingers.us
    service: http://localhost:9000
  - hostname: subtitler.pottingers.us
    service: http://localhost:8060
  - service: http_status:404
```

Restart cloudflared to pick up the new config:
```bash
sudo systemctl restart cloudflared
```

## Corrections to deployment.md

The existing `subtitler/specs/deployment.md` contains some incorrect assumptions:

| deployment.md Says | Reality |
|--------------------|---------|
| Build on Mac host with cross-compilation | Build natively on ARM64 VM |
| `CGO_ENABLED=1 GOOS=linux GOARCH=arm64` | `CGO_ENABLED=1 go build` (native, CGO required for sqlite) |
| SSH/SCP from host to VM | No SSH - webhook-deployer runs on VM |
| `rsync -avz dist/ vm:/var/www/subtitler/` | No rsync needed - Caddy serves from `frontend/dist/` |

The deployment spec was written assuming a "build on dev machine, deploy to VM" workflow. The actual architecture uses webhook-deployer running inside the VM, which simplifies deployment significantly.

## Testing Locally

### Simulate a webhook

```bash
# Create test payload
cat > /tmp/test-payload.json << 'EOF'
{
  "ref": "refs/heads/subtitler_v3",
  "repository": { "full_name": "tpott/pub_musings" },
  "pusher": { "name": "trevor" },
  "commits": [{
    "added": [],
    "modified": ["subtitler/frontend/src/pages/index.astro"],
    "removed": []
  }]
}
EOF

# Generate signature
SIGNATURE=$(echo -n "$(cat /tmp/test-payload.json)" | \
  openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | \
  awk '{print "sha256="$2}')

# Send test webhook
curl -X POST http://localhost:9000/webhook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: push" \
  -H "X-Hub-Signature-256: $SIGNATURE" \
  -d @/tmp/test-payload.json
```

## Rollback Strategy

### Frontend Rollback
```bash
# Caddy serves static files directly - just restore from git
cd /home/trevor/pub_musings/subtitler/frontend
git checkout HEAD~1
npm ci && npm run build
# Caddy serves directly from dist/ - no rsync needed
```

### Backend Rollback
```bash
# Restore previous binary (kept by deploy script)
cd /home/trevor/pub_musings/subtitler/backend
cp subtitler-prev subtitler
systemctl --user restart subtitler
```

### Full Git Rollback
```bash
cd /home/trevor/pub_musings
git log --oneline -5  # Find good commit
git checkout <commit-sha> -- subtitler/
# Then re-run deploy scripts
```
