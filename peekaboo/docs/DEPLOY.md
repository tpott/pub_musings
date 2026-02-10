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

See `.env.example` for a complete list with descriptions. Key production variables:

| Variable | Description |
|----------|-------------|
| `PORT` | Backend server port (default: 8080, production: 8070) |
| `WHISPER_SERVER_URL` | URL to whisper-server (e.g., `http://10.0.2.2:8765`) |
| `ANTHROPIC_API_KEY` | Anthropic API key for LLM intent recognition |
| `LLM_PROVIDER` | `anthropic` or `openai` (default: `anthropic`) |
| `ALLOWED_ORIGIN` | CORS origin (e.g., `https://peekaboo.example.com`) |
| `CSRF_SECRET` | HMAC key for CSRF tokens (random if unset; set for token persistence across restarts) |
| `HTTPS_ONLY` | Set `true` behind HTTPS proxy for secure cookies |
| `TRUSTED_USERS` | Comma-separated user IDs for admin endpoints |

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
cd backend && go build -o ../peekaboo
cd ..
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

## Email Configuration

Peekaboo uses [Resend](https://resend.com) for transactional email (verification and magic link emails). Without configuration, emails are logged to stdout instead of being sent.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `RESEND_API_KEY` | Resend API key (omit for dev mode) | - |
| `EMAIL_FROM` | Sender address | `noreply@peekaboo.pottingers.us` |
| `APP_URL` | Base URL for email links | `http://localhost:4321` |

### Production Setup

1. Create a [Resend account](https://resend.com) and verify your sending domain
2. Add the API key and app URL to your secrets:

```bash
sops secrets.enc.yaml
# Add:
#   RESEND_API_KEY: re_xxxxxxxxxxxxx
#   APP_URL: https://peekaboo.pottingers.us
```

3. Re-decrypt secrets to `.env` and restart the backend

### Development Mode

When `RESEND_API_KEY` is not set, the backend uses `LogEmailSender` which logs email details (recipient, token) to stdout. This is suitable for local development — check server logs for verification tokens and magic link tokens.

## Backup and Recovery

### Database Backup

The SQLite database at `data/peekaboo.db` contains user feedback and media set configurations. Back it up regularly to prevent data loss.

**Manual backup:**

```bash
cd /home/trevor/pub_musings/peekaboo
# Use SQLite .backup command for consistent backup while server runs
sqlite3 data/peekaboo.db ".backup data/peekaboo.db.bak"
```

**Automated daily backup with cron:**

```bash
# Edit crontab
crontab -e

# Add this line for daily backup at 2 AM
0 2 * * * sqlite3 /home/trevor/pub_musings/peekaboo/data/peekaboo.db ".backup /home/trevor/pub_musings/peekaboo/data/backups/peekaboo-$(date +\%Y\%m\%d).db"
```

Create the backups directory first:

```bash
mkdir -p /home/trevor/pub_musings/peekaboo/data/backups
```

**Backup retention (keep last 7 days):**

```bash
# Add to crontab after backup command (runs at 3 AM)
0 3 * * * find /home/trevor/pub_musings/peekaboo/data/backups -name "peekaboo-*.db" -mtime +7 -delete
```

### Database Recovery

**Restore from backup:**

```bash
# Stop the server first
systemctl --user stop peekaboo

# Replace database with backup
cp data/backups/peekaboo-YYYYMMDD.db data/peekaboo.db

# Restart server
systemctl --user start peekaboo
```

**Verify database integrity after restore:**

```bash
sqlite3 data/peekaboo.db "PRAGMA integrity_check;"
# Should output: ok
```

### What's Stored

The database contains:
- **concepts** table: animal names (cat, dog, duck, pig, chicken, cow)
- **media_sets** table: paths to media files for each concept
- **feedback** table: user feedback with ratings, messages, timestamps, and IP addresses

Media files in `data/media/` should be backed up separately or can be regenerated with `scripts/source-media.sh`.

## Viewing User Feedback

User feedback is stored in the SQLite database. There's no admin web interface, but you can query feedback directly using the `sqlite3` command-line tool.

### View Recent Feedback

```bash
cd /home/trevor/pub_musings/peekaboo
sqlite3 -header -column data/peekaboo.db "
  SELECT id, feedback_type, rating, message, created_at
  FROM feedback
  ORDER BY created_at DESC
  LIMIT 20;
"
```

### Export All Feedback to CSV

```bash
sqlite3 -header -csv data/peekaboo.db "
  SELECT id, feedback_type, rating, message, session_id, concept_id,
         transcript, page_url, user_agent, ip_address, created_at, status
  FROM feedback
  ORDER BY created_at DESC;
" > feedback-export.csv
```

### View Feedback Statistics

```bash
sqlite3 -header -column data/peekaboo.db "
  SELECT
    feedback_type,
    COUNT(*) as count,
    AVG(rating) as avg_rating,
    MIN(created_at) as first,
    MAX(created_at) as last
  FROM feedback
  GROUP BY feedback_type;
"
```

### View Feedback by Rating

```bash
# Show only low-rated feedback (for prioritizing issues)
sqlite3 -header -column data/peekaboo.db "
  SELECT id, feedback_type, rating, message, created_at
  FROM feedback
  WHERE rating IS NOT NULL AND rating <= 2
  ORDER BY created_at DESC;
"
```

### Update Feedback Status

Feedback has a `status` field (default: 'new') that can be used to track review progress:

```bash
# Mark feedback as reviewed
sqlite3 data/peekaboo.db "UPDATE feedback SET status = 'reviewed' WHERE id = 'feedback_xxx';"

# Mark feedback as resolved
sqlite3 data/peekaboo.db "UPDATE feedback SET status = 'resolved' WHERE id = 'feedback_xxx';"

# View unreviewed feedback
sqlite3 -header -column data/peekaboo.db "
  SELECT id, feedback_type, rating, message, created_at
  FROM feedback
  WHERE status = 'new'
  ORDER BY created_at DESC;
"
```

### Clear Old Feedback (90-day retention)

```bash
# Preview what will be deleted
sqlite3 data/peekaboo.db "
  SELECT COUNT(*) as to_delete FROM feedback
  WHERE created_at < datetime('now', '-90 days');
"

# Delete old feedback
sqlite3 data/peekaboo.db "
  DELETE FROM feedback
  WHERE created_at < datetime('now', '-90 days');
"
```

## Webhook Deployer

The webhook-deployer is configured in `../webhook-deployer/config.yaml` to deploy both frontend and backend when changes are pushed to the `peek1` branch.

Deploy scripts location:
- `../webhook-deployer/scripts/deploy-peekaboo-frontend.sh`
- `../webhook-deployer/scripts/deploy-peekaboo-backend.sh`
