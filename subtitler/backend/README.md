# Subtitler Backend

Go HTTP server for the Subtitler application. Handles video uploads, transcription via Whisper AI, subtitle generation, and user authentication.

## Quick Start

```bash
# Run the server
/home/trevor/go/bin/go run main.go
# Server runs on http://localhost:8080
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `MAX_UPLOAD_SIZE` | `500M` | Maximum upload file size (supports K, M, G suffixes) |
| `UPLOAD_DIR` | `uploads` | Directory for uploaded files |
| `DB_PATH` | `data/subtitler.db` | Path to SQLite database file |
| `KEY_PATH` | `data/age.key` | Path to age encryption key file |
| `WHISPER_MODEL` | `$HOME/Github/whisper.cpp/models/ggml-medium.bin` | Path to whisper model (CLI mode only) |
| `WHISPER_SERVER_URL` | `http://127.0.0.1:8765` | URL of whisper-server (enables server mode) |
| `USE_WHISPER_SERVER` | `false` | Set to `true` to use whisper-server with default URL |
| `RESEND_API_KEY` | *(none)* | Resend API key for transactional emails (starts with `re_`) |
| `EMAIL_FROM` | `noreply@subtitler.app` | Sender email address for outgoing emails |
| `APP_URL` | `http://localhost:4321` | Base URL for email links (e.g., password reset) |
| `EMAIL_ENABLED` | `true` | Set to `false` to disable email sending |
| `HTTPS_ONLY` | `false` | Set to `true` or `1` to enable Secure flag on session cookies |

### File Path Configuration

For production deployments, you'll typically want to configure these paths:

```bash
# Example production configuration
export UPLOAD_DIR="/opt/subtitler/uploads"
export DB_PATH="/opt/subtitler/data/subtitler.db"
export KEY_PATH="/opt/subtitler/data/age.key"
export MAX_UPLOAD_SIZE="1G"  # Increase limit for production
```

### Whisper Mode Selection

The backend supports two modes for speech-to-text transcription:

**CLI Mode (default):**
- Spawns `whisper-cli` subprocess for each transcription
- Model loaded from disk each time (slower)
- Requires whisper-cli in PATH and model file available

**Server Mode:**
- Sends audio to whisper-server HTTP API
- Model stays loaded in memory (faster)
- Supports remote transcription (GPU server)

Server mode is enabled when:
1. `WHISPER_SERVER_URL` is set to any URL, OR
2. `USE_WHISPER_SERVER=true` (uses default URL)

Example:
```bash
# CLI mode with custom model
export WHISPER_MODEL="/path/to/ggml-large-v3.bin"

# Server mode with default localhost URL
export USE_WHISPER_SERVER=true

# Server mode with remote server
export WHISPER_SERVER_URL="http://192.168.1.100:8765"
```

## Running whisper-server

Start whisper-server on the same machine or a separate GPU server:

```bash
cd ~/Github/whisper.cpp

# Build (if not already built)
cmake -B build && cmake --build build --config Release -j

# Run server
./build/bin/whisper-server \
  -m models/ggml-medium.bin \
  --host 0.0.0.0 \
  --port 8765 \
  --convert
```

Options:
- `-m`: Model file path
- `--host`: Bind address (`0.0.0.0` for external access, `127.0.0.1` for local only)
- `--port`: Port number (use 8765 to avoid conflict with backend's 8080)
- `--convert`: Auto-convert audio formats via ffmpeg
- `-t N`: Number of threads (default: 4)

## Deployment

For qemu VM deployment (running backend in VM with whisper-server on host), see [specs/deployment.md](../specs/deployment.md).

Quick reference for VM networking:
```bash
# In the VM, whisper-server on host is accessible at 10.0.2.2 (qemu gateway)
export WHISPER_SERVER_URL="http://10.0.2.2:8765"
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Health check |
| POST | `/api/upload` | Upload video file |
| POST | `/api/transcribe/{id}` | Start transcription |
| GET | `/api/transcribe/{id}` | Get transcription status |
| PUT | `/api/transcribe/{id}/segments` | Update subtitle segments |
| POST | `/api/transcribe/{id}/align` | Align pasted transcript |
| GET | `/api/videos` | List user's videos |
| GET | `/api/videos/{id}/video` | Stream video file |
| GET | `/api/videos/{id}/subtitles.srt` | Download SRT file |
| POST | `/api/videos/{id}/burn` | Start subtitle burning |
| GET | `/api/videos/{id}/burn` | Get burn job status |
| GET | `/api/videos/{id}/burned` | Download burned video |
| POST | `/api/auth/register` | Create account |
| POST | `/api/auth/login` | Login |
| POST | `/api/auth/logout` | Logout |
| GET | `/api/auth/me` | Get current user |
| POST | `/api/auth/totp/setup` | Setup 2FA |
| POST | `/api/auth/totp/verify` | Verify 2FA code |
| POST | `/api/auth/totp/disable` | Disable 2FA |
| POST | `/api/auth/totp/recover` | Use recovery code to disable 2FA |
| POST | `/api/auth/totp/codes` | Regenerate recovery codes |
| GET | `/api/auth/sessions` | List user's active sessions |
| DELETE | `/api/auth/sessions/{id}` | Revoke a session |
| POST | `/api/auth/forgot-password` | Request password reset email |
| POST | `/api/auth/reset-password` | Reset password with token |
| POST | `/api/log` | Frontend console log forwarding |

## Project Structure

```
backend/
├── main.go          # HTTP handlers and server setup
├── main_test.go     # SRT formatting tests
├── api_test.go      # API integration tests
├── align/           # Transcript alignment algorithm
├── auth/            # Authentication and sessions
├── crypto/          # File encryption (age)
├── db/              # SQLite database layer
├── email/           # Email service (Resend API)
├── ratelimit/       # Rate limiting middleware
├── script/          # Script detection and conversion
└── totp/            # TOTP 2FA implementation
```

## Testing

```bash
# Run all backend tests
/home/trevor/go/bin/go test ./... -v

# Run with coverage
/home/trevor/go/bin/go test ./... -cover

# Run specific package tests
/home/trevor/go/bin/go test ./auth -v
```

## See Also

- [../README.md](../README.md) - Project overview
- [../INSTALL.md](../INSTALL.md) - Installation guide
- [../TESTING.md](../TESTING.md) - Test documentation
- [../specs/auth.md](../specs/auth.md) - Auth specification
- [../specs/encryption.md](../specs/encryption.md) - Encryption specification
- [../specs/totp.md](../specs/totp.md) - TOTP specification
- [../specs/email.md](../specs/email.md) - Email service specification
