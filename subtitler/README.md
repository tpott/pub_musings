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

### TODO
* Install section and link to INSTALL.md
* Linters section and link to LINTERS.md
* Browser automation section and link to BROWSER_TESTING.md
* Debugging section, how to attach debuggers

## Install

TODO create [INSTALL.md](INSTALL.md) to document how to install dependencies for this project
