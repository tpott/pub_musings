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

# Tests (when implemented)
cd frontend && npm test
cd backend && /home/trevor/go/bin/go test ./...
```

## Reference

- See `personal/001_INITIALIZATION.md` for Resend API and sops+age patterns
- See `001_RALPH_SUBTITLER.md` for architecture and task overview
