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
| `WHISPER_SERVER_URL` | URL to whisper-server (e.g., `http://10.0.2.2:8765`) |
| `ANTHROPIC_API_KEY` | Anthropic API key for LLM intent recognition |

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
- Proxy `/api/*` to Go backend on port 8080

## Webhook Deployer

The webhook-deployer is configured in `../webhook-deployer/config.yaml` to deploy both frontend and backend when changes are pushed to the `peek1` branch.

Deploy scripts location:
- `../webhook-deployer/scripts/deploy-peekaboo-frontend.sh`
- `../webhook-deployer/scripts/deploy-peekaboo-backend.sh`
