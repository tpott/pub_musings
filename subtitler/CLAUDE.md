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

## Commands

```bash
# Frontend
cd frontend && npm install
cd frontend && npm run dev

# Backend
cd backend && go mod download
cd backend && go run ./cmd/server

# Tests
cd frontend && npm test
cd backend && go test ./...
```

## Reference

- See `personal/001_INITIALIZATION.md` for Resend API and sops+age patterns
- See `001_RALPH_SUBTITLER.md` for architecture and task overview
