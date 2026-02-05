# Deployment Guide

## Prerequisites

- `sops` installed (v3.9+)
- Age key at `~/.config/sops/age/keys.txt`
- Go 1.21+
- Node.js 18+

## Decrypting Secrets

Secrets are stored encrypted in `secrets.enc.yaml` using sops with age encryption.

### Decrypt to .env file

```bash
cd /home/trevor/pub_musings/peekaboo
sops -d secrets.enc.yaml > .env.tmp
# Convert YAML to shell format
grep -E '^[A-Z_]+:' .env.tmp | sed 's/: /=/' > .env
rm .env.tmp
```

Or use the one-liner:

```bash
sops -d secrets.enc.yaml | grep -E '^[A-Z_]+:' | sed 's/: /=/' > .env
```

### Edit secrets

```bash
sops secrets.enc.yaml
```

This opens the decrypted file in your editor. When you save and exit, sops re-encrypts it.

### Add new secrets

1. Edit the encrypted file: `sops secrets.enc.yaml`
2. Add new key-value pairs in YAML format
3. Save and exit - sops encrypts automatically

## Environment Variables

| Variable | Description |
|----------|-------------|
| `PORT` | Backend server port (default: 8080, production: 8070) |
| `WHISPER_SERVER_URL` | URL to whisper-server (e.g., `http://10.0.2.2:8765`) |
| `ANTHROPIC_API_KEY` | Anthropic API key for LLM intent recognition |

## Systemd Service Setup

Install the user service for automatic restarts:

```bash
cp docs/peekaboo.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable peekaboo
systemctl --user start peekaboo
```

Check status:

```bash
systemctl --user status peekaboo
journalctl --user -u peekaboo -f
```

## Deployment Steps

### 1. Pull latest code

```bash
cd /home/trevor/pub_musings
git pull origin peek1
```

### 2. Decrypt secrets

```bash
cd peekaboo
sops -d secrets.enc.yaml | grep -E '^[A-Z_]+:' | sed 's/: /=/' > .env
```

### 3. Build and run backend

```bash
go build -o peekaboo
./peekaboo
```

### 4. Build frontend

```bash
cd frontend
npm ci
npm run build
```

### 5. Serve via Caddy

Configure Caddy to serve:
- Frontend static files from `frontend/dist/`
- Proxy `/api/*`, `/ws/*`, `/health*`, and `/data/media/*` to Go backend

Example Caddyfile block:

```caddy
peekaboo.pottingers.us {
    # Logging - JSON format for structured log analysis
    log {
        output file /var/log/caddy/peekaboo-access.log
        format json
    }

    # Proxy API requests to Go backend with extended timeouts
    handle /api/* {
        reverse_proxy localhost:8070 {
            transport http {
                read_timeout 300s
                write_timeout 300s
            }
        }
    }

    # WebSocket endpoint for audio streaming
    handle /ws/* {
        reverse_proxy localhost:8070
    }

    # Health check endpoints
    handle /health {
        reverse_proxy localhost:8070
    }

    handle /health/* {
        reverse_proxy localhost:8070
    }

    # Media files (photos, audio) served by backend
    handle /data/media/* {
        reverse_proxy localhost:8070
    }

    # Serve static frontend files
    handle {
        root * /home/trevor/pub_musings/peekaboo/frontend/dist
        file_server
        encode gzip

        # Cache static assets (1 year, immutable)
        @static path *.js *.css *.png *.jpg *.svg *.woff2
        header @static Cache-Control "public, max-age=31536000, immutable"

        # Don't cache HTML
        @html path *.html /
        header @html Cache-Control "no-cache"
    }
}
```

> **Note:** The port (8070) should match the `PORT` environment variable configured in `peekaboo.service`. Production may use a different port (e.g., 9070).

## HTTPS and TLS

**IMPORTANT: Peekaboo requires HTTPS in production.**

The frontend uses the MediaRecorder API for microphone access, which browsers only allow on secure contexts (HTTPS or localhost). Running over plain HTTP will cause microphone access to silently fail.

### Caddy Auto-HTTPS

Caddy automatically provisions and renews TLS certificates via Let's Encrypt. When you configure a domain in Caddyfile (e.g., `peekaboo.pottingers.us`), Caddy:

1. Requests a certificate from Let's Encrypt
2. Configures HTTPS with modern cipher suites
3. Redirects HTTP to HTTPS automatically
4. Renews certificates before expiration

Requirements for auto-HTTPS:
- Domain DNS must point to your server
- Ports 80 and 443 must be accessible
- Email for Let's Encrypt can be set globally: `email admin@example.com`

### Certificate Storage

Caddy stores certificates at:
- Linux: `~/.local/share/caddy/certificates/`
- macOS: `~/Library/Application Support/Caddy/certificates/`

### Development (localhost)

For local development, HTTPS is not required. Browsers allow microphone access on `localhost` without TLS. Run the frontend dev server normally:

```bash
cd frontend && npm run dev
```

### Security Warnings

- **Never run HTTP in production** - microphone access will fail and user data (voice recordings, transcripts) will be transmitted unencrypted
- **Don't use self-signed certificates** - browsers will show warnings and may block microphone access
- **Verify certificate validity** - test with `curl -v https://peekaboo.example.com/health`

## Webhook Deployer

The webhook-deployer is configured in `../webhook-deployer/config.yaml` to deploy both frontend and backend when changes are pushed to the `peek1` branch.

Deploy scripts location:
- `../webhook-deployer/scripts/deploy-peekaboo-frontend.sh`
- `../webhook-deployer/scripts/deploy-peekaboo-backend.sh`
