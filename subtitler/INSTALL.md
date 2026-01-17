# Installation Guide

This guide covers setting up the Subtitler development environment.

## Prerequisites

### Required Software

1. **Node.js and npm**
   - Version: Node.js 18.x or higher
   - Check version: `node --version && npm --version`
   - Install: https://nodejs.org/

2. **Go**
   - Version: Go 1.22 or higher
   - Check version: `go version`
   - Install: https://go.dev/dl/
   - If Go is not in system PATH, it may be installed at `/home/trevor/go/bin/go`

3. **whisper.cpp** (for transcription)
   - Required for Task 2 and beyond
   - Clone from: https://github.com/ggerganov/whisper.cpp
   - Build instructions: See Task 2 plan when implemented

4. **ffmpeg** (for video processing)
   - Required for subtitle embedding
   - Install: `sudo apt-get install ffmpeg` (Ubuntu/Debian)
   - Or download from: https://ffmpeg.org/download.html

### Optional (for deployment)

- **Cloudflare account** - For Cloudflare Tunnel setup (Task 11)
- **sops and age** - For secrets management (Task 13)

## Setup Instructions

### 1. Clone the Repository

```bash
cd /home/trevor/pub_musings
# Repository should already be cloned
```

### 2. Frontend Setup

```bash
cd /home/trevor/pub_musings/subtitler/frontend
npm install
```

Verify installation:
```bash
npm run dev
```

Frontend should be accessible at http://localhost:4321

### 3. Backend Setup

```bash
cd /home/trevor/pub_musings/subtitler/backend
go mod download
```

Verify installation:
```bash
go build ./cmd/server
./server
# Or run directly: go run ./cmd/server
```

Backend should start on http://localhost:8080

Test health endpoint:
```bash
curl http://localhost:8080/health
# Should return: OK
```

### 4. Verify Structure

After setup, your directory should look like:

```
subtitler/
├── frontend/
│   ├── node_modules/
│   ├── src/
│   ├── package.json
│   └── package-lock.json
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── auth/auth.go
│   │   ├── transcribe/transcribe.go
│   │   └── storage/storage.go
│   └── go.mod
└── ...
```

## Running the Application

### Development Mode

Terminal 1 (Frontend):
```bash
cd frontend
npm run dev
```

Terminal 2 (Backend):
```bash
cd backend
go run ./cmd/server
```

### Build for Production

Frontend:
```bash
cd frontend
npm run build
```

Backend:
```bash
cd backend
go build -o subtitler-server ./cmd/server
./subtitler-server
```

## Environment Variables

Environment variables will be configured in later tasks. For now, no environment variables are required.

Future environment variables:
- Database connection string
- File storage encryption key
- Resend API key for email
- Session secret key

## Troubleshooting

### Go not found

If `go` command is not found, ensure it's in your PATH:
```bash
export PATH=$PATH:/home/trevor/go/bin
```

Or use the full path:
```bash
/home/trevor/go/bin/go version
```

### npm install fails

Try clearing npm cache:
```bash
npm cache clean --force
rm -rf node_modules package-lock.json
npm install
```

### Port already in use

If port 8080 or 4321 is already in use, you can modify:
- Backend: Change `port` variable in `backend/cmd/server/main.go`
- Frontend: Use `npm run dev -- --port 3000` or modify `astro.config.mjs`

## Next Steps

After successful installation:
1. Review [README.md](README.md) for project overview
2. Check [TASKS.jsonl](TASKS.jsonl) for implementation progress
3. See [001_RALPH_SUBTITLER.md](001_RALPH_SUBTITLER.md) for architecture details
