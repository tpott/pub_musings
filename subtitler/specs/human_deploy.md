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

# Deploy to web root
sudo rsync -av --delete dist/ /var/www/subtitler/

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
curl http://localhost:8080/  # Or through Caddy

# Check backend running
systemctl status subtitler
curl http://localhost:8080/api/health
```

## VM Prerequisites
The deploy script needs `sudo` access for:
- `rsync` to `/var/www/subtitler/`
- `cp` to `/opt/subtitler/backend/`
- `systemctl restart subtitler`

Ensure the `trevor` user has passwordless sudo for these commands, or add to sudoers:
```
trevor ALL=(ALL) NOPASSWD: /usr/bin/rsync, /bin/cp, /bin/systemctl restart subtitler
```

## Corrections to deployment.md

The existing `subtitler/specs/deployment.md` contains some incorrect assumptions:

| deployment.md Says | Reality |
|--------------------|---------|
| Build on Mac host with cross-compilation | Build natively on ARM64 VM |
| `CGO_ENABLED=1 GOOS=linux GOARCH=arm64` | Just `go build` (native) |
| SSH/SCP from host to VM | No SSH - webhook-deployer runs on VM |
| `rsync -avz dist/ vm:/var/www/subtitler/` | `rsync` locally on VM |

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
sudo rsync -av --delete dist/ /var/www/subtitler/
```

### Backend Rollback
```bash
# Restore previous binary (kept by deploy script)
sudo cp /opt/subtitler/backend/subtitler-prev /opt/subtitler/backend/subtitler
sudo systemctl restart subtitler
```

### Full Git Rollback
```bash
cd /home/trevor/pub_musings
git log --oneline -5  # Find good commit
git checkout <commit-sha> -- subtitler/
# Then re-run deploy scripts
```
