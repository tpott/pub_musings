# Deployment Plan

This document describes the deployment architecture for the Subtitler application on a Mac Mini with qemu virtualization.

## Architecture Overview

```
┌────────────────────────────────────────────────────────────────────────┐
│                          Mac Mini M1 (Host)                            │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐ │
│  │                     qemu VM (Ubuntu ARM64)                        │ │
│  │                                                                   │ │
│  │   ┌────────────────┐     ┌──────────────┐                        │ │
│  │   │  cloudflared   │────▶│    Caddy     │───▶ Backend (8080)     │ │
│  │   │(tunnel client) │     │  (443/80)    │                        │ │
│  │   └────────────────┘     └──────┬───────┘                        │ │
│  │                                 │ Static files                    │ │
│  │                                 ▼                                 │ │
│  │                          /var/www/subtitler/                      │ │
│  │                                                                   │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│          │                                                             │
│          │ VM accesses host via 10.0.2.2 (qemu user-mode gateway)     │
│          ▼                                                             │
│  ┌──────────────────────────────────────────────────────────────────┐ │
│  │                    whisper-server (port 8765)                     │ │
│  │              (Metal GPU-accelerated on host)                      │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘

External traffic: Cloudflare → cloudflared tunnel → Caddy → Backend
```

## Components

### 1. qemu VM (Linux Guest)

Runs the web application in an isolated environment.

**Specifications:**
- OS: Ubuntu Server 24.04 LTS (ARM64)
- vCPUs: 2-4
- RAM: 4-8 GB
- Disk: 50 GB (qcow2)
- Network: User-mode networking (NAT)

**qemu launch command:**
```bash
qemu-system-aarch64 \
  -name subtitler-vm \
  -machine virt,accel=hvf \
  -cpu host \
  -smp cores=4 \
  -m 8G \
  -drive file=subtitler.qcow2,format=qcow2,if=virtio \
  -netdev user,id=net0,hostfwd=tcp::2222-:22 \
  -device virtio-net,netdev=net0 \
  -nographic
```

**Port forwarding:**
- Host `2222` → VM `22` (SSH access)
- No HTTP/HTTPS ports needed - traffic comes through Cloudflare tunnel

**Networking notes:**
- With qemu user-mode networking, the VM can access the host at IP `10.0.2.2`
- This is qemu's default gateway for the virtual NAT network
- The backend uses `http://10.0.2.2:8765` to reach whisper-server on the host

### 2. Cloudflare Tunnel (cloudflared)

Routes external traffic into the VM without exposing ports publicly.

**Installation (Ubuntu ARM64):**
```bash
# Download cloudflared for ARM64
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64 -o /usr/local/bin/cloudflared
chmod +x /usr/local/bin/cloudflared
```

**Setup:**
```bash
# Authenticate (one-time)
cloudflared tunnel login

# Create tunnel
cloudflared tunnel create subtitler

# Configure tunnel
cat > ~/.cloudflared/config.yml << 'EOF'
tunnel: <tunnel-id>
credentials-file: /home/subtitler/.cloudflared/<tunnel-id>.json

ingress:
  - hostname: subtitler.example.com
    service: http://localhost:80
  - service: http_status:404
EOF
```

**Service file (`/etc/systemd/system/cloudflared.service`):**
```ini
[Unit]
Description=Cloudflare Tunnel
After=network.target

[Service]
Type=simple
User=subtitler
ExecStart=/usr/local/bin/cloudflared tunnel run subtitler
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**DNS:**
Configure DNS in Cloudflare dashboard to point `subtitler.example.com` to the tunnel.

### 3. Caddy Web Server

Serves static frontend files and proxies API requests to the backend.

**Caddyfile:**
```caddyfile
subtitler.example.com {
    # Serve static frontend files
    root * /var/www/subtitler
    file_server

    # Proxy API requests to Go backend
    handle /api/* {
        reverse_proxy localhost:8080
    }

    # SPA fallback - serve index.html for client-side routes
    try_files {path} /index.html

    # Security headers
    header {
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        Referrer-Policy strict-origin-when-cross-origin
    }

    # Compression
    encode gzip

    # Logging
    log {
        output file /var/log/caddy/access.log
        format json
    }
}
```

**Installation (Ubuntu):**
```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy
```

### 4. Go Backend

Runs as a systemd service.

**Service file (`/etc/systemd/system/subtitler.service`):**
```ini
[Unit]
Description=Subtitler Backend
After=network.target

[Service]
Type=simple
User=subtitler
Group=subtitler
WorkingDirectory=/opt/subtitler/backend
ExecStart=/opt/subtitler/backend/subtitler
Restart=always
RestartSec=5

# Environment
Environment=PORT=8080
Environment=WHISPER_SERVER_URL=http://10.0.2.2:8765
Environment=UPLOAD_DIR=/opt/subtitler/uploads
Environment=DB_PATH=/opt/subtitler/data/subtitler.db
Environment=KEY_PATH=/opt/subtitler/data/age.key
Environment=MAX_UPLOAD_SIZE=1G
Environment=HTTPS_ONLY=true
Environment=TRUST_PROXY=true
Environment=DB_MAINTENANCE_INTERVAL=24h

# Security
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/subtitler/data /opt/subtitler/uploads

[Install]
WantedBy=multi-user.target
```

**Commands:**
```bash
sudo systemctl enable subtitler
sudo systemctl start subtitler
sudo systemctl status subtitler
```

### 5. whisper-server (Host)

Runs on the Mac Mini host for GPU acceleration.

**Launch script (`~/bin/start-whisper-server.sh`):**
```bash
#!/bin/bash
cd ~/Github/whisper.cpp
./build/bin/whisper-server \
  -m models/ggml-large-v3-turbo.bin \
  --host 0.0.0.0 \
  --port 8765 \
  --convert \
  -t 8
```

**launchd plist (`~/Library/LaunchAgents/com.user.whisper-server.plist`):**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.user.whisper-server</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/trevor/bin/start-whisper-server.sh</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/Users/trevor/Library/Logs/whisper-server.log</string>
    <key>StandardErrorPath</key>
    <string>/Users/trevor/Library/Logs/whisper-server.error.log</string>
</dict>
</plist>
```

**Commands:**
```bash
launchctl load ~/Library/LaunchAgents/com.user.whisper-server.plist
launchctl start com.user.whisper-server
```

## Directory Structure

```
/opt/subtitler/
├── backend/
│   ├── subtitler          # Compiled Go binary
│   ├── data/
│   │   ├── subtitler.db   # SQLite database
│   │   └── age.key        # Encryption key
│   └── uploads/           # Encrypted video files
└── frontend/              # (symlink to /var/www/subtitler)

/var/www/subtitler/        # Astro static build output
├── index.html
├── upload/index.html
├── videos/index.html
├── login/index.html
├── register/index.html
├── security/index.html
└── _astro/                # JS/CSS assets
```

## Deployment Workflow

### Initial Setup

1. **Create VM:**
   ```bash
   # On Mac Mini host
   qemu-img create -f qcow2 subtitler.qcow2 50G
   # Install Ubuntu Server via ISO
   ```

2. **Configure VM:**
   ```bash
   # In VM
   sudo useradd -r -s /bin/false subtitler
   sudo mkdir -p /opt/subtitler/{backend/data,backend/uploads}
   sudo mkdir -p /var/www/subtitler
   sudo chown -R subtitler:subtitler /opt/subtitler
   ```

3. **Install dependencies:**
   ```bash
   # In VM
   sudo apt update
   sudo apt install -y caddy ffmpeg
   ```

4. **Configure Caddy:**
   ```bash
   sudo vim /etc/caddy/Caddyfile
   sudo systemctl reload caddy
   ```

5. **Deploy backend:**
   ```bash
   # Build on dev machine
   CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -o subtitler

   # Copy to VM
   scp subtitler vm:/opt/subtitler/backend/

   # Start service
   sudo systemctl start subtitler
   ```

6. **Deploy frontend:**
   ```bash
   # Build on dev machine
   cd frontend && npm run build

   # Copy to VM
   rsync -avz dist/ vm:/var/www/subtitler/
   ```

### Updating Deployments

See [webhook-deployer integration](../specs/webhook-deployer.md) for automated deployments.

**Manual update:**
```bash
# Backend
scp subtitler vm:/opt/subtitler/backend/
ssh vm 'sudo systemctl restart subtitler'

# Frontend
rsync -avz dist/ vm:/var/www/subtitler/
# No restart needed - Caddy serves static files
```

## SSL/TLS Certificates

With Cloudflare tunnel, TLS is handled by Cloudflare. Caddy runs on HTTP inside the VM.

For local/dev deployments without tunnel:
```caddyfile
localhost {
    tls internal
    # ... rest of config
}
```

## qemu Networking Details

### Understanding 10.0.2.2

In qemu user-mode networking, the VM gets a private network:
- **10.0.2.2** - The host machine (qemu's virtual gateway)
- **10.0.2.3** - DNS server (forwarded to host)
- **10.0.2.15** - VM's default IP

The VM can access any service running on the host via `10.0.2.2`. No port forwarding needed for outbound connections (VM→host).

### Alternative: Bridge Networking

For more advanced setups where VM needs direct LAN access:

```bash
# On host: Create bridge (one-time setup)
sudo ip link add br0 type bridge
sudo ip link set br0 up
sudo ip addr add 192.168.100.1/24 dev br0

# qemu with bridge
qemu-system-aarch64 \
  -netdev bridge,id=net0,br=br0 \
  -device virtio-net,netdev=net0 \
  ...
```

Configure VM:
```bash
sudo ip addr add 192.168.100.2/24 dev eth0
export WHISPER_SERVER_URL="http://192.168.100.1:8765"
```

### Firewall Configuration

Ensure host firewall allows whisper-server connections from VM:

```bash
# For user-mode networking
sudo ufw allow from 10.0.2.0/24 to any port 8765

# For bridge networking
sudo ufw allow from 192.168.100.0/24 to any port 8765
```

### Verifying Connectivity

From inside the VM:
```bash
# Test whisper-server health endpoint
curl http://10.0.2.2:8765/health
# Expected: {"status":"ok"}

# Test inference (requires a WAV file)
curl http://10.0.2.2:8765/inference \
  -F file=@test.wav \
  -F response_format=json
```

## Monitoring

### Health Checks

```bash
# Check backend
curl http://localhost:8080/api/health

# Check whisper-server (from host)
curl http://localhost:8765/health

# Check whisper-server (from VM)
curl http://10.0.2.2:8765/health
```

### Logs

```bash
# Caddy logs
sudo journalctl -u caddy -f

# Backend logs
sudo journalctl -u subtitler -f

# whisper-server logs (on host)
tail -f ~/Library/Logs/whisper-server.log
```

## Backup Strategy

### Database Backup

```bash
# Backup SQLite database
sqlite3 /opt/subtitler/backend/data/subtitler.db ".backup '/backup/subtitler-$(date +%Y%m%d).db'"

# Or use rsync
rsync -av /opt/subtitler/backend/data/ /backup/subtitler-data/
```

### Encryption Key Backup

**Critical:** Back up `data/age.key` securely. Without it, encrypted files are unrecoverable.

```bash
# Copy to secure location
cp /opt/subtitler/backend/data/age.key /backup/age.key
chmod 600 /backup/age.key
```

## Security Considerations

1. **Firewall:** Only expose ports 80/443 to the internet
2. **Backend:** Not directly accessible (only via Caddy proxy)
3. **whisper-server:** Only accessible from VM (firewall host:8765 from external)
4. **Files:** All uploads encrypted at rest with age
5. **Database:** SQLite in WAL mode, regular backups
6. **Secrets:** Encryption key secured with restricted permissions

## Future Improvements

1. **Docker containerization:** Package backend + frontend in containers for easier deployment
2. **Kubernetes:** For horizontal scaling when needed
3. **CDN:** CloudFlare or similar for static asset caching
4. **Monitoring:** Prometheus + Grafana for metrics
5. **GPU passthrough:** See [Metal via MoltenVK research](../specs/metal-moltenvk.md)

## See Also

- [../backend/README.md](../backend/README.md) - Backend environment variables
- [../INSTALL.md](../INSTALL.md) - Development installation
- [webhook-deployer integration](webhook-deployer.md) - Automated deployments
- [Metal via MoltenVK research](metal-moltenvk.md) - GPU passthrough research
