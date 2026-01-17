# Subtitler Project

Web-based subtitling service using whisper.cpp for transcription.

## Override: Install Commands

The parent `pub_musings/CLAUDE.md` says "do not run install commands." This project's CLAUDE.md **overrides that rule** when running in the Ralph loop (`--dangerously-skip-permissions`).

When running as Ralph, you MAY:
- Run install commands (`npm install`, `go get`, etc.)
- Execute builds and tests
- Run the development servers

## Key Constraints

| Constraint | Value |
|------------|-------|
| File size limit | ~10 minutes of 1080p MP4 video |
| Database | SQLite (local) |
| File storage | Local, encrypted with age (separate key from env vars) |
| Email provider | Resend API |

## Project Structure

```
subtitler/
├── frontend/           # Astro frontend (TypeScript)
│   ├── src/
│   ├── public/
│   ├── package.json
│   └── astro.config.mjs
├── backend/            # Go backend API
│   ├── cmd/
│   │   └── server/     # Main server entry point
│   ├── internal/
│   │   ├── auth/       # Authentication (Task 6)
│   │   ├── transcribe/ # Whisper.cpp integration (Task 2)
│   │   └── storage/    # File storage (Task 5)
│   └── go.mod
├── v1/                 # Previous Python implementation (archived)
├── TASKS.jsonl         # Ralph task tracking
└── *.md                # Documentation files
```

## Environment Setup

### Go Installation
Go is installed at `/home/trevor/go/bin/go` (version 1.22.10).

Use full path or add to PATH:
```bash
export PATH=$PATH:/home/trevor/go/bin
```

### Node/npm
Frontend uses Node.js with npm. Version 18.x or higher required.

### whisper.cpp Installation
whisper.cpp is installed at `~/Github/whisper.cpp/`.

**Key paths:**
- whisper-server binary: `~/Github/whisper.cpp/build/bin/whisper-server`
- Medium model: `~/Github/whisper.cpp/models/ggml-medium.bin`
- Sample audio: `~/Github/whisper.cpp/samples/jfk.wav`

**Configuration:**
The backend uses the following default configuration for whisper.cpp integration (see `internal/config/config.go`):
- `WHISPER_SERVER_PATH`: `$HOME/Github/whisper.cpp/build/bin/whisper-server`
- `WHISPER_MODEL_PATH`: `$HOME/Github/whisper.cpp/models/ggml-medium.bin`
- `WHISPER_SERVER_PORT`: 9090
- `WHISPER_THREADS`: 4

Override via environment variables if needed.

**How it works:**
- The backend starts whisper-server as a subprocess when the transcription service starts
- whisper-server loads the model once at startup
- Transcription requests are sent to whisper-server via HTTP (localhost:9090)
- whisper-server handles audio format conversion using ffmpeg
- The subprocess is stopped gracefully when the service shuts down

## Commands

```bash
# Frontend
cd frontend && npm install
cd frontend && npm run dev        # Runs on http://localhost:4321

# Backend
cd backend && /home/trevor/go/bin/go mod download
cd backend && /home/trevor/go/bin/go run ./cmd/server  # Runs on http://localhost:8080

# Build backend
cd backend && /home/trevor/go/bin/go build ./cmd/server

# Tests
cd frontend && npm test  # (when implemented)

# Unit tests (fast)
cd backend && /home/trevor/go/bin/go test ./... -short

# Integration tests (requires whisper.cpp)
cd backend && /home/trevor/go/bin/go test ./internal/transcribe -v
```

## Reference

- See `personal/001_INITIALIZATION.md` for Resend API and sops+age patterns
- See `001_RALPH_SUBTITLER.md` for architecture and task overview
