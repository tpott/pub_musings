# Subtitler Deployment Configuration

This directory contains configuration files for deploying Subtitler to production via CI/CD pipeline.

## Overview

- **Backend:** Go API server with whisper.cpp integration, runs as systemd service
- **Frontend:** Astro static site served by Caddy
- **Deployment:** GitHub webhook triggers automated deployment script
- **Tunnel:** Cloudflare Tunnel exposes services to internet

## Files

| File | Purpose | Destination on VM |
|------|---------|-------------------|
| `subtitler-backend.service` | systemd service for backend | `/etc/systemd/system/subtitler-backend.service` |
| `Caddyfile.example` | Caddy web server config | Merge into `/etc/caddy/Caddyfile` |
| `cloudflared-config.example.yml` | Cloudflare Tunnel config | Merge into `/etc/cloudflared/config.yml` |
| `sudoers-subtitler-deploy` | Sudo permissions for deployment | `/etc/sudoers.d/subtitler-deploy` |

## Prerequisites

Before deploying, ensure:

- [x] Ubuntu VM with SSH access
- [x] Go 1.22.10+ installed at `/home/trevor/go/bin/go`
- [x] Node.js 18+ with npm
- [x] whisper.cpp built at `~/Github/whisper.cpp/`
- [x] Caddy installed and running
- [x] Cloudflare Tunnel created and authenticated
- [x] Domain names added to Cloudflare DNS

## Initial Setup (One-Time)

Run these commands on the VM:

### 1. Create Backend .env File

```bash
cd ~/pub_musings/subtitler/backend

# Option A: If secrets.enc.yaml exists, decrypt it
sops -d ~/pub_musings/subtitler/secrets.enc.yaml | \
  grep -E "^(JWT_SECRET|RESEND_API_KEY|EMAIL_FROM|FRONTEND_URL):" | \
  sed 's/: /=/' | sed 's/"//g' > .env

# Option B: Create manually
cat > .env << 'EOF'
# Server
SERVER_PORT=8080
JWT_SECRET=<generate-with: openssl rand -base64 32>
FRONTEND_URL=https://subtitler.yourdomain.com
COOKIE_SECURE=true

# Database
DATABASE_PATH=/home/trevor/pub_musings/subtitler/data/db/subtitler.db
DATA_DIR=/home/trevor/pub_musings/subtitler/data

# Email
RESEND_API_KEY=re_xxxxx
EMAIL_FROM=noreply@yourdomain.com
ENABLE_EMAIL=true

# Whisper.cpp
WHISPER_MODEL_PATH=/home/trevor/Github/whisper.cpp/models/ggml-medium.bin
WHISPER_SERVER_PATH=/home/trevor/Github/whisper.cpp/build/bin/whisper-server
WHISPER_SERVER_PORT=9090
WHISPER_THREADS=4
EOF

# Secure permissions
chmod 600 .env
```

### 2. Create Frontend .env File

```bash
cd ~/pub_musings/subtitler/frontend

# Option A: If secrets.enc.yaml exists, decrypt it
sops -d ~/pub_musings/subtitler/secrets.enc.yaml | \
  grep -E "^PUBLIC_API_URL:" | \
  sed 's/: /=/' | sed 's/"//g' > .env

# Option B: Create manually
cat > .env << 'EOF'
PUBLIC_API_URL=https://api.subtitler.yourdomain.com
EOF

chmod 600 .env
```

### 3. Install systemd Service

```bash
sudo cp ~/pub_musings/subtitler/deploy/subtitler-backend.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable subtitler-backend
```

### 4. Configure Sudo Permissions

```bash
sudo cp ~/pub_musings/subtitler/deploy/sudoers-subtitler-deploy /etc/sudoers.d/subtitler-deploy
sudo chmod 440 /etc/sudoers.d/subtitler-deploy

# Test it works (should not prompt for password)
sudo systemctl status subtitler-backend
```

### 5. Update Caddy Configuration

```bash
# Backup existing config
sudo cp /etc/caddy/Caddyfile /etc/caddy/Caddyfile.backup

# Add Subtitler configuration (merge with existing config)
sudo nano /etc/caddy/Caddyfile
# Copy contents from deploy/Caddyfile.example

# Test and reload
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

### 6. Update Cloudflare Tunnel Configuration

```bash
# Backup existing config
sudo cp /etc/cloudflared/config.yml /etc/cloudflared/config.yml.backup

# Add Subtitler routes (merge with existing ingress rules)
sudo nano /etc/cloudflared/config.yml
# Copy ingress rules from deploy/cloudflared-config.example.yml

# Add DNS routes
cloudflared tunnel route dns <tunnel-name> subtitler.yourdomain.com
cloudflared tunnel route dns <tunnel-name> api.subtitler.yourdomain.com

# Restart tunnel
sudo systemctl restart cloudflared
```

### 7. Initial Build and Start

```bash
cd ~/pub_musings/subtitler

# Build frontend
cd frontend && npm ci && npm run build

# Build backend
cd ../backend && /home/trevor/go/bin/go build -o subtitler-server ./cmd/server

# Start service
sudo systemctl start subtitler-backend
sudo systemctl status subtitler-backend

# Check logs
journalctl -u subtitler-backend -f
```

## Verification

After setup, verify all components:

```bash
# Backend service is running
sudo systemctl status subtitler-backend
# Expected: active (running)

# Backend health check (local)
curl http://localhost:8080/api/health
# Expected: {"success":true,"message":"Service is healthy","version":"1.0.0"}

# Frontend served by Caddy (local)
curl http://localhost:8081/
# Expected: HTML content

# Backend via Caddy proxy (local)
curl http://localhost:8082/api/health
# Expected: {"success":true,"message":"Service is healthy","version":"1.0.0"}

# Public endpoints via Cloudflare Tunnel
curl https://subtitler.yourdomain.com/
# Expected: HTML content

curl https://api.subtitler.yourdomain.com/api/health
# Expected: {"success":true,"message":"Service is healthy","version":"1.0.0"}
```

## GitHub Webhook Setup

1. Go to GitHub repository → Settings → Webhooks → Add webhook
2. **Payload URL:** `https://webhook.yourdomain.com/webhook`
3. **Content type:** `application/json`
4. **Secret:** Use `WEBHOOK_SECRET` from secrets.enc.yaml
5. **Events:** Just the push event
6. **Active:** ✓

## Automated Deployment

Once configured, deployments are automatic:

```bash
# Local machine
git add .
git commit -m "Your changes"
git push origin trunk

# GitHub sends webhook → webhook-deployer runs deploy-subtitler.sh
# Script: git pull → npm build → go build → systemctl restart
```

Monitor deployment:

```bash
# SSH to VM
journalctl -u webhook-deployer -f    # Webhook handler logs
journalctl -u subtitler-backend -f   # Backend service logs
```

## Manual Deployment

If needed, deploy manually:

```bash
cd ~/pub_musings/subtitler
./deploy-subtitler.sh
```

## Rollback

If deployment fails:

```bash
cd ~/pub_musings/subtitler
git log --oneline -10  # Find previous good commit
git reset --hard <commit-hash>
./deploy-subtitler.sh
```

## Troubleshooting

### Backend service won't start

```bash
# Check logs
sudo journalctl -u subtitler-backend -n 100

# Common issues:
# - Missing .env file → Create backend/.env
# - Wrong permissions on .env → chmod 600 backend/.env
# - whisper.cpp not found → Check WHISPER_SERVER_PATH in .env
# - Database migration failed → Check DATA_DIR permissions
```

### Frontend 404 errors

```bash
# Check Caddy is serving files
ls -la ~/pub_musings/subtitler/frontend/dist/
curl http://localhost:8081/

# Common issues:
# - dist/ doesn't exist → Run npm run build
# - Caddy can't access directory → chmod o+x ~
# - Wrong path in Caddyfile → Check root directive
```

### Cloudflare Tunnel not routing

```bash
# Check tunnel status
sudo systemctl status cloudflared
journalctl -u cloudflared -n 50

# Verify DNS routes
cloudflared tunnel route ip show

# Common issues:
# - Wrong hostname in config.yml → Check ingress rules
# - DNS not configured → Run cloudflared tunnel route dns
# - Service not listening → Check backend service status
```

### Deployment script fails

```bash
# Check deploy script permissions
ls -la ~/pub_musings/subtitler/deploy-subtitler.sh
# Should be: -rwxr-xr-x (executable)

# Run manually to see errors
cd ~/pub_musings/subtitler
./deploy-subtitler.sh

# Common issues:
# - npm ci fails → Delete node_modules and package-lock.json, try again
# - go build fails → Check go.mod is valid
# - systemctl restart fails → Check sudoers configuration
```

## Security Notes

- **.env files:** Never commit to git, always `chmod 600`
- **JWT_SECRET:** Generate with `openssl rand -base64 32`
- **WEBHOOK_SECRET:** Must match GitHub webhook configuration
- **Age key:** Store private key securely (password manager)
- **Sudoers:** Restricted to specific systemctl commands only

## Maintenance

### Update Dependencies

```bash
# Frontend
cd frontend && npm update && npm audit fix

# Backend
cd backend && /home/trevor/go/bin/go get -u ./...
```

### View Logs

```bash
# Last 100 lines
journalctl -u subtitler-backend -n 100

# Follow live logs
journalctl -u subtitler-backend -f

# Logs since 1 hour ago
journalctl -u subtitler-backend --since "1 hour ago"
```

### Database Backup

```bash
# Backup SQLite database
cp ~/pub_musings/subtitler/data/db/subtitler.db ~/backups/subtitler-$(date +%Y%m%d).db

# Verify backup
sqlite3 ~/backups/subtitler-*.db "SELECT count(*) FROM users;"
```

## References

- [Caddy Documentation](https://caddyserver.com/docs/)
- [Cloudflare Tunnel Docs](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/)
- [systemd Service Documentation](https://www.freedesktop.org/software/systemd/man/systemd.service.html)
- [sops + age Setup](../../../personal/001_INITIALIZATION.md#secrets-management-with-sops--age)
