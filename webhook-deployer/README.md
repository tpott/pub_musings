# Webhook Deployer

A Go HTTP service that handles:
1. GitHub webhook deployments for the personal website
2. Contact form submissions with email via Resend

## Setup

Assuming you already have `go` installed and setup in your `$PATH`, you should run:
```bash
go get .
```

to install the dependencies for this project.

## Building

```bash
go build -o webhook-deployer .
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `WEBHOOK_SECRET` | Yes | GitHub webhook secret for signature validation |
| `RESEND_API_KEY` | Yes | Resend API key for sending emails |
| `EMAIL_FROM` | Yes | From address for contact form emails (e.g., `Contact Form <noreply@example.com>`) |
| `EMAIL_TO` | Yes | Recipient address for contact form emails (e.g., `you@example.com`) |
| `ALLOWED_ORIGIN` | Yes | Production origin for CORS (e.g., `https://example.com`) |
| `SITE_PATH` | No | Path to Astro site (default: `/home/trevor/pub_musings/personal`) |
| `PORT` | No | HTTP port (default: `9000`) |

**Note:** `http://localhost:4321` and `http://127.0.0.1:4321` are always allowed as CORS origins for local development.

## Endpoints

- `POST /webhook` - GitHub webhook receiver
- `POST /api/contact` - Contact form handler
- `GET /health` - Health check

## GitHub Webhook Setup

1. Go to your repository Settings > Webhooks
2. Add webhook:
   - Payload URL: `https://webhook.pottingers.us/webhook`
   - Content type: `application/json`
   - Secret: Your `WEBHOOK_SECRET`
   - Events: Just the push event

## Running as a systemd Service

Create `/etc/systemd/system/webhook-deployer.service`:

```ini
[Unit]
Description=GitHub Webhook Deployer
After=network.target

[Service]
Type=simple
User=trevor
WorkingDirectory=/home/trevor
ExecStart=/home/trevor/pub_musings/webhook-deployer/webhook-deployer
EnvironmentFile=/home/trevor/pub_musings/webhook-deployer/.env
Restart=always

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable webhook-deployer
sudo systemctl start webhook-deployer
```

View logs:

```bash
journalctl -u webhook-deployer -f
```

## Rate Limiting

The contact form is rate-limited to 5 requests per IP per hour with a burst of 2. This uses an in-memory store that cleans up entries older than 3 hours.

## Spam Prevention

- Honeypot field (`website`) - if filled, the request is silently "accepted" to fool bots
- Rate limiting per IP using Cloudflare's `CF-Connecting-IP` header
