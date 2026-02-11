# Agent Instructions

This file contains instructions for AI agents working on this project.

## Override: Install Commands

The parent `pub_musings/CLAUDE.md` says "do not run install commands." This project's CLAUDE.md **overrides that rule** when running in the Ralph loop (`--dangerously-skip-permissions`).

When running as Ralph, you MAY:
- Run install commands (`npm install`, `go get`, etc.)
- Execute builds and tests
- Run the development servers

## Commands

### Go Backend

```bash
# Build (from backend/ directory)
cd backend && go build ./...

# Run all tests
cd backend && go test ./...

# Run with verbose output
cd backend && go test -v ./...

# Run a specific package's tests
cd backend && go test ./api
cd backend && go test ./crypto
cd backend && go test ./db
cd backend && go test ./email
cd backend && go test ./llm
cd backend && go test ./tts

# Start the server (requires env vars)
cd backend && go run main.go
```

### Frontend (Astro)

```bash
# Install dependencies (from frontend/ directory)
cd frontend && npm install

# Run development server
cd frontend && npm run dev

# Run unit tests
cd frontend && npm test

# Build for production
cd frontend && npm run build
```

### E2E Tests (Playwright)

```bash
# Install Playwright browsers (one time)
cd frontend && npx playwright install

# Run e2e tests
cd frontend && npx playwright test

# Run e2e tests with UI
cd frontend && npx playwright test --ui
```

### Media Scripts

```bash
# Download CC0 media assets
./scripts/source-media.sh

# Encrypt media files (requires age to be installed)
./scripts/encrypt-media.sh --generate-key  # First time
./scripts/encrypt-media.sh                 # Subsequent runs
./scripts/encrypt-media.sh --remove-originals  # Production
```

### Secrets Management (sops)

```bash
# Decrypt secrets to .env (requires age key)
# Converts YAML format (KEY: value) to shell format (KEY=value)
sops -d secrets.enc.yaml | grep -E '^[A-Z_]+:' | sed 's/: /=/' > .env
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP server port | `8080` |
| `DB_PATH` | SQLite database path | `data/peekaboo.db` |
| `DB_MAX_OPEN_CONNS` | Max open database connections | `1` |
| `DB_MAX_IDLE_CONNS` | Max idle database connections | `1` |
| `MEDIA_DIR` | Media files directory | `data/media` |
| `AGE_KEY_FILE` | Age encryption key file | `data/age.key` |
| `LLM_PROVIDER` | LLM provider (`anthropic` or `openai`) | `anthropic` |
| `ANTHROPIC_API_KEY` | Anthropic API key | - |
| `OPENAI_API_KEY` | OpenAI API key | - |
| `WHISPER_SERVER_URL` | Whisper server URL | `http://127.0.0.1:8765` |
| `PIPER_SERVER_URL` | Piper TTS server URL (optional) | - |
| `TRUST_PROXY_HEADERS` | Trust X-Forwarded-For/X-Real-IP for IP extraction | `false` |
| `ALLOWED_ORIGIN` | CORS allowed origin (e.g., `https://peekaboo.example.com`) | `*` (dev only) |
| `WEBSOCKET_IDLE_TIMEOUT_SECS` | WebSocket idle timeout in seconds | `300` |
| `WEBSOCKET_MAX_CONNECTIONS` | Max concurrent WebSocket connections | `100` |
| `LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) | `info` |
| `LOG_FORMAT` | Log format (`text` or `json`) | `text` |
| `FORWARD_FRONTEND_LOGS` | Enable frontend log forwarding (dev only) | - |
| `CSRF_SECRET` | HMAC key for CSRF tokens (hex or raw string) | random |
| `HTTPS_ONLY` | Set Secure flag on cookies | `false` |
| `INTERACTION_LOG_AUDIO` | Save audio blobs to disk for interaction logs | `false` |
| `INTERACTION_RETENTION_DAYS` | Days to retain interaction audio blobs before cleanup | `30` |
| `RESEND_API_KEY` | Resend API key for sending emails (omit for LogEmailSender) | - |
| `EMAIL_FROM` | Sender email address for transactional emails | `noreply@peekaboo.pottingers.us` |
| `APP_URL` | Base URL for email links (verification, magic link) | `http://localhost:4321` |
| `TRUSTED_USERS` | Comma-separated user IDs for admin endpoints | - |
| `PROD_HOST` | Production host URL (for scripts/fetch-feedback.py) | - |
| `API_SESSION_ID` | Session token for admin scripts | - |

> **Note on DB connections:** SQLite only supports one writer at a time, even with WAL mode. The default of 1 connection is recommended. Higher values may improve read performance but can cause "database is locked" errors on write-heavy workloads.

## Failed Commands Log

Document commands that failed unexpectedly during development here:

(None recorded yet)
