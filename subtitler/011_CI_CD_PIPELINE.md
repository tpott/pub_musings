# 011 CI/CD Pipeline Implementation

## Overview

Implement a GitHub webhook-based CI/CD pipeline for automated deployments of the Subtitler service. This follows the same pattern as the personal site but adapted for a full-stack Go + Astro application with background workers.

## Architecture

**Domain Structure** (TBD - placeholder domains used):
- Frontend: `https://subtitler.yourdomain.com`
- Backend API: `https://api.subtitler.yourdomain.com`
- Webhook: `https://webhook.yourdomain.com`

```
┌─────────────┐  webhook.yourdomain.com  ┌──────────────────────────────────────────┐
│   GitHub    │────────────────────────▶│              Ubuntu VM                   │
│  (trunk)    │                         │  ┌─────────────────────────────────────┐ │
└─────────────┘                         │  │  webhook-deployer (Go, port 9000)   │ │
                                        │  │  - validates signature              │ │
                                        │  │  - runs: deploy-subtitler.sh        │ │
                                        │  └─────────────────────────────────────┘ │
                                        │                                          │
┌─────────────┐   tunnel                │  ┌─────────────────────────────────────┐ │
│ Cloudflare  │◀────────────────────────│  │  cloudflared                        │ │
│   Edge      │  subtitler...:8081      │  │  - subtitler.yourdomain.com → :8081 │ │
└─────────────┘  api.subtitler...:8080  │  │  - api.subtitler.yourdomain.com     │ │
       │         webhook...:9000         │  │    → :8080                           │ │
       ▼                                │  │  - webhook.yourdomain.com → :9000   │ │
   Internet                             │  └─────────────────────────────────────┘ │
                                        │                    │                     │
                                        │                    ▼                     │
                                        │  ┌─────────────────────────────────────┐ │
                                        │  │  Caddy (port 8081)                  │ │
                                        │  │  - serves frontend /dist files      │ │
                                        │  │  - proxies API requests to :8080    │ │
                                        │  └─────────────────────────────────────┘ │
                                        │                                          │
                                        │  ┌─────────────────────────────────────┐ │
                                        │  │  Go Backend (port 8080)             │ │
                                        │  │  - API server with worker pool      │ │
                                        │  │  - whisper.cpp integration          │ │
                                        │  └─────────────────────────────────────┘ │
                                        │                                          │
                                        │  ┌─────────────────────────────────────┐ │
                                        │  │  /home/trevor/pub_musings/subtitler/│ │
                                        │  │  ├── frontend/dist/                 │ │
                                        │  │  ├── backend/server (binary)        │ │
                                        │  │  └── data/ (SQLite DB, files)       │ │
                                        │  └─────────────────────────────────────┘ │
                                        └──────────────────────────────────────────┘
```

## Deployment Flow

1. **Developer pushes to trunk** → GitHub sends webhook to `https://webhook.yourdomain.com/webhook`
2. **webhook-deployer validates signature** → Executes `deploy-subtitler.sh`
3. **deploy-subtitler.sh**:
   - Pulls latest code from `trunk`
   - Builds frontend (Astro)
   - Builds backend (Go)
   - Restarts backend service (systemd)
   - Caddy serves new frontend files immediately (no restart needed)

## Implementation Steps

### Step 1: Create Deployment Script

**File:** `subtitler/deploy-subtitler.sh`

```bash
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
```

**Make executable:**
```bash
chmod +x deploy-subtitler.sh
```

### Step 2: Create Backend systemd Service

**File:** `/etc/systemd/system/subtitler-backend.service`

```ini
[Unit]
Description=Subtitler Backend API Server
After=network.target

[Service]
Type=simple
User=trevor
WorkingDirectory=/home/trevor/pub_musings/subtitler/backend
ExecStart=/home/trevor/pub_musings/subtitler/backend/subtitler-server
EnvironmentFile=/home/trevor/pub_musings/subtitler/backend/.env
Restart=always
RestartSec=5

# Resource limits
LimitNOFILE=65536

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=subtitler-backend

[Install]
WantedBy=multi-user.target
```

**Setup commands:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable subtitler-backend
sudo systemctl start subtitler-backend
sudo systemctl status subtitler-backend
```

### Step 3: Configure Caddy

**File:** `/etc/caddy/Caddyfile`

```
# Frontend - Static files
:8081 {
    root * /home/trevor/pub_musings/subtitler/frontend/dist
    file_server
    encode gzip

    # Cache static assets
    @static path *.js *.css *.png *.jpg *.svg *.woff2 *.woff *.ttf
    header @static Cache-Control "public, max-age=31536000, immutable"

    # Don't cache HTML
    @html path *.html /
    header @html Cache-Control "no-cache"

    # SPA fallback (if needed)
    try_files {path} /index.html
}

# Backend API - Reverse proxy
:8082 {
    reverse_proxy localhost:8080
}
```

**Apply configuration:**
```bash
sudo systemctl reload caddy
```

### Step 4: Update Cloudflare Tunnel Configuration

**File:** `/etc/cloudflared/config.yml`

```yaml
tunnel: <tunnel-id>
credentials-file: /home/trevor/.cloudflared/<tunnel-id>.json

ingress:
  # Frontend
  - hostname: subtitler.yourdomain.com
    service: http://localhost:8081

  # Backend API
  - hostname: api.subtitler.yourdomain.com
    service: http://localhost:8082

  # Webhook (already configured for personal site)
  - hostname: webhook.yourdomain.com
    service: http://localhost:9000

  # Default 404
  - service: http_status:404
```

**Add DNS routes:**
```bash
cloudflared tunnel route dns <tunnel-name> subtitler.yourdomain.com
cloudflared tunnel route dns <tunnel-name> api.subtitler.yourdomain.com
```

**Reload tunnel:**
```bash
sudo systemctl restart cloudflared
```

### Step 5: Configure webhook-deployer

The existing webhook-deployer service needs to be updated to handle subtitler deployments.

**Update:** `pub_musings/webhook-deployer/webhook.go`

Add new repository handler:

```go
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    // ... existing signature validation ...

    repo := payload.Repository.FullName
    branch := payload.Ref

    switch repo {
    case "username/pub_musings":  // Replace with your GitHub username
        if branch == "refs/heads/trunk" {
            // Determine which project changed
            if hasChangesInPath(payload.Commits, "subtitler/") {
                err = runDeployScript("/home/trevor/pub_musings/subtitler/deploy-subtitler.sh")
            } else if hasChangesInPath(payload.Commits, "personal/") {
                err = runDeployScript("/home/trevor/pub_musings/personal/deploy.sh")
            }
            // ... error handling ...
        }
    default:
        http.Error(w, "Repository not configured", http.StatusBadRequest)
        return
    }
}

func hasChangesInPath(commits []Commit, pathPrefix string) bool {
    for _, commit := range commits {
        for _, file := range append(commit.Added, append(commit.Modified, commit.Removed...)...) {
            if strings.HasPrefix(file, pathPrefix) {
                return true
            }
        }
    }
    return false
}
```

### Step 6: Environment Configuration

**File:** `subtitler/backend/.env` (create on VM, not committed)

```bash
# Database
DATABASE_PATH=/home/trevor/pub_musings/subtitler/data/db/subtitler.db
DATA_DIR=/home/trevor/pub_musings/subtitler/data

# Server
SERVER_PORT=8080
JWT_SECRET=<generate-secure-random-secret>
FRONTEND_URL=https://subtitler.yourdomain.com

# Email (Resend)
RESEND_API_KEY=re_xxxxx
EMAIL_FROM=noreply@yourdomain.com
ENABLE_EMAIL=true

# Whisper.cpp
WHISPER_MODEL_PATH=/home/trevor/Github/whisper.cpp/models/ggml-medium.bin
WHISPER_SERVER_PATH=/home/trevor/Github/whisper.cpp/build/bin/whisper-server
WHISPER_SERVER_PORT=9090
WHISPER_THREADS=4
```

**File:** `subtitler/frontend/.env` (create on VM, not committed)

```bash
PUBLIC_API_URL=https://api.subtitler.yourdomain.com
```

### Step 7: Secrets Management with sops + age

Following the same pattern as personal site:

**File:** `subtitler/secrets.enc.yaml` (encrypted, committed)

```yaml
# These will be encrypted with sops
WEBHOOK_SECRET: "your-github-webhook-secret"
JWT_SECRET: "<generate-random-secret>"
RESEND_API_KEY: "re_xxxxx"
EMAIL_FROM: "noreply@yourdomain.com"
FRONTEND_URL: "https://subtitler.yourdomain.com"
PUBLIC_API_URL: "https://api.subtitler.yourdomain.com"
```

**Encryption workflow:**
```bash
# Create plaintext file (never commit)
cat > subtitler/secrets.yaml << 'EOF'
WEBHOOK_SECRET: "..."
JWT_SECRET: "..."
RESEND_API_KEY: "..."
EMAIL_FROM: "..."
FRONTEND_URL: "..."
PUBLIC_API_URL: "..."
EOF

# Encrypt with sops (uses .sops.yaml config from repo root)
sops -e subtitler/secrets.yaml > subtitler/secrets.enc.yaml
rm subtitler/secrets.yaml  # Delete plaintext

# Commit encrypted version
git add subtitler/secrets.enc.yaml
```

**Decryption on VM:**
```bash
# Decrypt to backend .env
sops -d ~/pub_musings/subtitler/secrets.enc.yaml | \
  grep -E "^(JWT_SECRET|RESEND_API_KEY|EMAIL_FROM|FRONTEND_URL):" | \
  sed 's/: /=/' | sed 's/"//g' \
  >> ~/pub_musings/subtitler/backend/.env

# Add static config
cat >> ~/pub_musings/subtitler/backend/.env << 'EOF'
DATABASE_PATH=/home/trevor/pub_musings/subtitler/data/db/subtitler.db
DATA_DIR=/home/trevor/pub_musings/subtitler/data
SERVER_PORT=8080
ENABLE_EMAIL=true
WHISPER_MODEL_PATH=/home/trevor/Github/whisper.cpp/models/ggml-medium.bin
WHISPER_SERVER_PATH=/home/trevor/Github/whisper.cpp/build/bin/whisper-server
WHISPER_SERVER_PORT=9090
WHISPER_THREADS=4
EOF

# Decrypt to frontend .env
sops -d ~/pub_musings/subtitler/secrets.enc.yaml | \
  grep -E "^PUBLIC_API_URL:" | \
  sed 's/: /=/' | sed 's/"//g' \
  > ~/pub_musings/subtitler/frontend/.env

# Secure permissions
chmod 600 ~/pub_musings/subtitler/backend/.env
chmod 600 ~/pub_musings/subtitler/frontend/.env
```

### Step 8: Sudoers Configuration

The `trevor` user needs sudo permissions to restart the backend service.

**File:** `/etc/sudoers.d/subtitler-deploy`

```
trevor ALL=(ALL) NOPASSWD: /bin/systemctl restart subtitler-backend
trevor ALL=(ALL) NOPASSWD: /bin/systemctl status subtitler-backend
trevor ALL=(ALL) NOPASSWD: /bin/systemctl is-active subtitler-backend
trevor ALL=(ALL) NOPASSWD: /usr/bin/journalctl -u subtitler-backend *
```

**Set permissions:**
```bash
sudo chmod 440 /etc/sudoers.d/subtitler-deploy
```

### Step 9: GitHub Webhook Configuration

1. Go to repository settings → Webhooks → Add webhook
2. **Payload URL:** `https://webhook.yourdomain.com/webhook`
3. **Content type:** `application/json`
4. **Secret:** Use the `WEBHOOK_SECRET` from secrets.enc.yaml
5. **Events:** Just the push event
6. **Active:** Check the box

## Verification Steps

### Local Testing (Before Deployment)

```bash
# Test frontend build
cd frontend && npm run build
ls -la dist/  # Should see index.html and assets

# Test backend build
cd backend && /home/trevor/go/bin/go build -o subtitler-server ./cmd/server
./subtitler-server  # Should start without errors (Ctrl+C to stop)

# Test deployment script locally
cd subtitler
./deploy-subtitler.sh  # Should complete without errors
```

### On VM (After Setup)

```bash
# Verify backend service is running
sudo systemctl status subtitler-backend
curl http://localhost:8080/api/health
# Expected: {"success":true,"message":"Service is healthy","version":"1.0.0"}

# Verify Caddy serves frontend
curl http://localhost:8081/
# Expected: HTML content

# Verify tunnel endpoints
curl https://api.subtitler.yourdomain.com/api/health
curl https://subtitler.yourdomain.com/
```

### End-to-End Deployment Test

```bash
# Make a small change
echo "# Test deployment" >> README.md
git add README.md
git commit -m "Test CI/CD pipeline"
git push origin trunk

# Monitor webhook logs
ssh vm
journalctl -u webhook-deployer -f

# Monitor backend deployment
journalctl -u subtitler-backend -f

# Verify deployment succeeded
curl https://api.subtitler.yourdomain.com/api/health
```

## Done When

✅ `git push to trunk` triggers GitHub webhook
✅ New version deploys automatically to VM
✅ Backend service restarts successfully
✅ Frontend static files updated
✅ Both `https://subtitler.yourdomain.com` and `https://api.subtitler.yourdomain.com` serve updated code

## Files Created

**In subtitler/ (committed):**
- [ ] `deploy-subtitler.sh` - Deployment script
- [ ] `secrets.enc.yaml` - Encrypted secrets (sops)

**On VM (not committed):**
- [ ] `/etc/systemd/system/subtitler-backend.service` - Backend service
- [ ] `/etc/sudoers.d/subtitler-deploy` - Sudo permissions for deployment
- [ ] `/etc/caddy/Caddyfile` - Updated with subtitler routes
- [ ] `/etc/cloudflared/config.yml` - Updated with subtitler DNS routes
- [ ] `subtitler/backend/.env` - Decrypted backend secrets
- [ ] `subtitler/frontend/.env` - Decrypted frontend secrets

**In webhook-deployer/ (updated):**
- [ ] `webhook.go` - Updated to handle subtitler deployments

## Security Notes

1. **JWT_SECRET:** Generate using `openssl rand -base64 32`
2. **WEBHOOK_SECRET:** Generate using `openssl rand -base64 32`
3. **File permissions:** Ensure `.env` files are `chmod 600` (readable only by owner)
4. **Sudoers:** Limited to specific systemctl commands for subtitler-backend only
5. **Age key:** Store private key in password manager (1Password, Bitwarden)

## Rollback Plan

If deployment fails:

```bash
# Stop broken service
sudo systemctl stop subtitler-backend

# Rollback to previous commit
cd ~/pub_musings/subtitler
git log --oneline -10  # Find previous good commit
git reset --hard <commit-hash>

# Redeploy
./deploy-subtitler.sh
```

## Future Enhancements

1. **Health checks:** Add health check before completing deployment
2. **Slack notifications:** Send deployment status to Slack channel
3. **Rolling deploys:** Zero-downtime deployments with blue-green pattern
4. **Database migrations:** Add migration verification step
5. **Smoke tests:** Run basic API tests after deployment
6. **Deployment logs:** Store deployment logs in dedicated directory

## Dependencies

- [x] Cloudflare account with tunnel setup (Task 11 - blocked)
- [x] Domain name configured in Cloudflare DNS
- [x] Ubuntu VM with SSH access
- [x] webhook-deployer service running (from personal site setup)
- [x] sops + age installed on VM and local machine

## Notes

- This CI/CD setup assumes Task 11 (Cloudflare Tunnel) is unblocked
- The webhook-deployer service is shared between personal site and subtitler
- Backend workers run in the same process as the API server (no separate worker service needed)
- whisper.cpp must be installed and built on the VM before deployment
