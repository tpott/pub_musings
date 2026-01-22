# subtitler

A web application for generating accurate subtitles for video content.

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
cd backend && /home/trevor/go/bin/go test ./... -v

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
