# Deployment Plan

This document describes the deployment architecture for the Subtitler application on a Mac Mini with qemu virtualization.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                          Mac Mini (Host)                             │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                     qemu VM (Linux)                          │   │
│  │                                                              │   │
│  │   ┌──────────────┐      ┌──────────────┐                    │   │
│  │   │    Caddy     │─────▶│   Backend    │                    │   │
│  │   │   (443/80)   │ /api │   (8080)     │                    │   │
│  │   └──────┬───────┘      └──────────────┘                    │   │
│  │          │                                                   │   │
│  │          │ Static files                                      │   │
│  │          ▼                                                   │   │
│  │   ┌──────────────┐                                          │   │
│  │   │ /var/www/    │                                          │   │
│  │   │ subtitler/   │                                          │   │
│  │   └──────────────┘                                          │   │
│  │                                                              │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                              │                                       │
│                              │ Host access (10.0.2.2:8765)          │
│                              ▼                                       │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                    whisper-server                             │   │
│  │              (GPU-accelerated on host)                        │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Components

### 1. qemu VM (Linux Guest)

Runs the web application in an isolated environment.

**Specifications:**
- OS: Ubuntu Server 24.04 LTS (minimal)
- vCPUs: 2-4
- RAM: 4-8 GB
- Disk: 50 GB (qcow2)
- Network: User-mode networking (NAT)

**qemu launch command:**
```bash
qemu-system-x86_64 \
  -name subtitler-vm \
  -machine type=q35,accel=hvf \
  -cpu host \
  -smp cores=4 \
  -m 8G \
  -drive file=subtitler.qcow2,format=qcow2,if=virtio \
  -netdev user,id=net0,hostfwd=tcp::8443-:443,hostfwd=tcp::8080-:80 \
  -device virtio-net,netdev=net0 \
  -nographic
```

Port forwarding:
- Host `8443` → VM `443` (HTTPS)
- Host `8080` → VM `80` (HTTP redirect)

### 2. Caddy Web Server

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

### 3. Go Backend

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

# Security
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/subtitler/backend/data /opt/subtitler/backend/uploads

[Install]
WantedBy=multi-user.target
```

**Commands:**
```bash
sudo systemctl enable subtitler
sudo systemctl start subtitler
sudo systemctl status subtitler
```

### 4. whisper-server (Host)

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
   CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o subtitler

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

Caddy automatically obtains and renews Let's Encrypt certificates when:
1. Domain is publicly accessible
2. DNS A record points to server
3. Ports 80/443 are open

For local/dev deployments, use self-signed:
```caddyfile
localhost {
    tls internal
    # ... rest of config
}
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
