# subtitler

A web application for generating accurate subtitles for video content.

## Features

### Core Functionality
- **Video Upload & Transcription**: Upload videos (up to 500MB) and generate subtitles using Whisper AI
- **Chunked Uploads**: Large files are automatically split into 50MB chunks for reliable uploading
- **Multiple Export Formats**: Download subtitles as SRT, VTT, or JSON
- **Subtitle Editor**: Edit text and timing with inline editing and undo/redo support
- **Subtitle Burning**: Embed subtitles directly into videos (burned or soft subtitles)

### Language & Script Support
- **Language Detection**: Automatic language detection with hints from video metadata and filename patterns
- **Language Override**: Select transcription language before or after processing
- **Script Conversion**: Convert romanized text to native scripts (9 Indic languages supported)
- **Lyrics Mode**: Special alignment mode for music videos with known lyrics

### User Experience
- **Variable Playback Speed**: Adjust speed (0.8x, 0.9x, 1x) for language learning
- **Bionic Reading Mode**: Bold first portion of words to aid reading speed (configurable in Settings)
- **Dark Mode**: Toggle between light, dark, and auto (system) themes
- **Keyboard Shortcuts**: Comprehensive shortcuts for video control and navigation
- **Video Thumbnails**: Preview thumbnails on the My Videos page

### Advanced Features
- **Embedded Subtitle Detection**: Automatically detects existing subtitle tracks in uploaded videos
- **Feedback System**: Built-in feedback button on all pages to report issues
- **Estimated Time Remaining**: Processing time estimates based on historical data

### Security
- **Authentication**: Email/password, magic link (passwordless), and TOTP 2FA
- **File Encryption**: Videos encrypted at rest using age library
- **Session Management**: View and revoke active sessions
- **CAPTCHA Protection**: Optional hCaptcha integration
- **Rate Limiting**: Protection against abuse on all endpoints

## Quick Start

### Backend (Go)

```bash
cd backend
go run main.go
# Server starts on http://localhost:8080
```

### Frontend (Astro)

```bash
cd frontend
npm install
npm run dev
# Server starts on http://localhost:4321
```

Visit http://localhost:4321 in your browser. The frontend proxies `/api/*` requests to the backend.

## Architecture

- **Frontend**: Astro (TypeScript) on port 4321
- **Backend**: Go on port 8080
- Frontend proxies `/api/*` to backend

### API Documentation

See [docs/API.md](docs/API.md) for complete API documentation including:
- All endpoints with request/response schemas
- Authentication methods (session cookie, Bearer token)
- Rate limiting information
- Example curl commands

### Environment Variables

See [docs/ENV.md](docs/ENV.md) for complete environment variable reference including:
- Server, storage, and Whisper configuration
- Email and security settings
- Rate limit configuration
- Example configurations for development and production

## Development

### Testing

See [TESTING.md](TESTING.md) for comprehensive testing documentation including:
- Running backend tests (Go)
- Running frontend tests (Vitest)
- Writing new tests
- Coverage reports

Quick test commands:
```bash
# Backend
cd backend && go test ./... -v

# Frontend
cd frontend && npm test
```

### Installation

See [INSTALL.md](INSTALL.md) for detailed instructions on installing:
- Node.js (v18+)
- Go (v1.22+)
- FFmpeg
- whisper-cli (from whisper.cpp)

### Linting

See [LINTERS.md](LINTERS.md) for linting setup:
- Go: golangci-lint, go fmt, go vet
- Frontend: ESLint, Prettier

### Browser Testing

See [BROWSER_TESTING.md](BROWSER_TESTING.md) for E2E testing with Playwright.

### Keyboard Shortcuts

The subtitle editor supports keyboard shortcuts for efficient video editing:

**Video Playback:**
| Shortcut | Action |
|----------|--------|
| `Space` | Play / Pause |
| `←` | Seek backward 5 seconds |
| `→` | Seek forward 5 seconds |
| `J` | Rewind 10 seconds |
| `K` | Pause |
| `L` | Forward 10 seconds |
| `[` | Slower playback speed |
| `]` | Faster playback speed |

**Subtitle Navigation:**
| Shortcut | Action |
|----------|--------|
| `Tab` | Jump to next segment |
| `Shift+Tab` | Jump to previous segment |

**Editing (in edit mode):**
| Shortcut | Action |
|----------|--------|
| `Ctrl+Z` | Undo |
| `Ctrl+Shift+Z` | Redo |

Press `?` to show the keyboard shortcuts help modal in the app.

### Debugging

#### Go Backend (Delve)

Install Delve debugger:
```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

**Run with debugger:**
```bash
cd backend
dlv debug . --
# or with full path: /home/trevor/go/bin/dlv debug .
```

**Attach VS Code:**
1. Create `.vscode/launch.json`:
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Backend",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/backend",
      "cwd": "${workspaceFolder}/backend"
    }
  ]
}
```
2. Press F5 in VS Code to start debugging

**Remote debugging (attach to running server):**
```bash
cd backend
dlv debug . --headless --listen=:2345 --api-version=2
# Then attach from VS Code or dlv connect :2345
```

#### Frontend (TypeScript)

**Browser DevTools:**
- Open http://localhost:4321, press F12
- Sources tab shows TypeScript files (with source maps)
- Console logs are forwarded to backend in dev mode (see terminal)

**VS Code:**
1. Install "Debugger for Chrome" or use built-in Edge debugger
2. Add to `.vscode/launch.json`:
```json
{
  "name": "Frontend (Chrome)",
  "type": "chrome",
  "request": "launch",
  "url": "http://localhost:4321",
  "webRoot": "${workspaceFolder}/frontend/src"
}
```
3. Start frontend (`npm run dev`), then press F5

**Console log forwarding:**

In development mode, frontend console.log/error/warn calls are forwarded to the backend terminal via `/api/log`. This helps debug frontend issues without switching windows.

### Production Debugging

For deployed environments running as systemd services (see [specs/deployment.md](specs/deployment.md)):

#### Service Status

```bash
# Check backend service status
sudo systemctl status subtitler

# Check Caddy web server
sudo systemctl status caddy

# Check Cloudflare tunnel (if used)
sudo systemctl status cloudflared
```

#### Log Locations

**Backend logs (subtitler.service):**
```bash
# Live logs
sudo journalctl -u subtitler -f

# Recent logs (last 100 lines)
sudo journalctl -u subtitler -n 100

# Logs since specific time
sudo journalctl -u subtitler --since "2024-01-26 10:00:00"

# Error-level logs only
sudo journalctl -u subtitler -p err
```

**Caddy logs:**
```bash
# Access logs (JSON format)
sudo tail -f /var/log/caddy/access.log | jq .

# Caddy service logs
sudo journalctl -u caddy -f
```

**whisper-server logs (on Mac host):**
```bash
# Standard output
tail -f ~/Library/Logs/whisper-server.log

# Error log
tail -f ~/Library/Logs/whisper-server.error.log
```

#### Common Issues

**Backend not starting:**
```bash
# Check for port conflict
sudo lsof -i :8080

# Check environment variables
sudo systemctl show subtitler | grep Environment

# View detailed startup failure
sudo journalctl -u subtitler -e
```

**Database locked errors:**
```bash
# Check SQLite lock file
ls -la /opt/subtitler/data/subtitler.db*

# Check for zombie processes
pgrep -a subtitler
```

**whisper-server connection failures:**
```bash
# Test from VM to host (if using qemu)
curl http://10.0.2.2:8765/health

# Test from localhost
curl http://localhost:8765/health

# Check if server is running (on host)
pgrep -a whisper-server
launchctl list | grep whisper
```

**Upload/transcription issues:**
```bash
# Check disk space
df -h /opt/subtitler/uploads

# Check file permissions
ls -la /opt/subtitler/uploads/
ls -la /opt/subtitler/data/

# Find recent uploads
find /opt/subtitler/uploads -mmin -60 -type f
```

#### Health Check

```bash
# Comprehensive health check
curl -s http://localhost:8080/api/health | jq .

# Expected output:
# {
#   "status": "ok",
#   "db_connected": true,
#   "whisper_available": true,
#   "disk_space_ok": true
# }
```

#### Request Tracing

All requests include an `X-Request-ID` header for tracing. Look for it in logs:

```bash
# Find specific request ID in logs
sudo journalctl -u subtitler | grep "abc123"

# Include request IDs in Caddy access logs
# (already configured in Caddyfile)
```

See also: [specs/deployment.md](specs/deployment.md) for full deployment documentation.
