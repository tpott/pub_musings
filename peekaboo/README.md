# Peekaboo

A voice-controlled web app for children that responds to prompts like "show me a cat" by displaying photos/videos and playing audio. It functions similarly to a Google Home speaker but with visual output.

## Features

- **Voice Input** - Press button, speak "show me a cat", release
- **Speech-to-Text** - Audio forwarded to whisper-server for transcription
- **Intent Recognition** - LLM extracts subject from natural language (supports Anthropic and OpenAI)
- **Media Display** - Shows curated CC0/public domain photos and plays animal sounds
- **Mobile-First** - Designed for touch devices

## Architecture

```
Browser (mobile-first)
    │
    │ /api/transcribe, /api/intent, /api/media/*
    ▼
Go Backend
    ├── SQLite DB (concepts, media_sets)
    ├── LLM Provider (Anthropic/OpenAI)
    └── Whisper Client
            │
            ▼
        whisper-server (external)
```

## Quick Start

### Prerequisites

- Go 1.21+
- Node.js 18+
- age (for encryption)
- Access to whisper-server
- Anthropic or OpenAI API key

### Setup

1. Clone and install dependencies:
   ```bash
   cd backend && go mod download
   cd ../frontend && npm install
   ```

2. Download media assets:
   ```bash
   ./scripts/source-media.sh
   ```

3. (Optional) Encrypt media:
   ```bash
   ./scripts/encrypt-media.sh --generate-key
   ```

4. Create `.env` file with required variables:
   ```bash
   cp .env.example .env
   # Edit .env with your API keys
   ```

5. Start the backend:
   ```bash
   cd backend && go run main.go
   ```

6. Start the frontend (in another terminal):
   ```bash
   cd frontend && npm run dev
   ```

7. Open http://localhost:4321 in your browser

## Project Structure

```
peekaboo/
├── backend/          # Go backend
│   ├── main.go       # Entry point
│   ├── api/          # HTTP handlers
│   │   ├── intent.go     # POST /api/intent - LLM intent extraction
│   │   ├── media.go      # GET /api/media/{concept} - Media lookup
│   │   ├── transcribe.go # POST /api/transcribe - Whisper forwarding
│   │   └── encrypted_media.go # Encrypted file serving
│   ├── crypto/       # Age encryption utilities
│   ├── db/           # SQLite database
│   └── llm/          # LLM provider abstraction (Anthropic/OpenAI)
├── frontend/         # Astro frontend
│   ├── src/lib/      # Core TypeScript modules
│   └── tests/        # Playwright e2e tests
├── scripts/          # Build and setup scripts
├── specs/            # Feature specifications
├── docs/             # Documentation
└── data/             # Runtime data (gitignored)
    ├── media/        # Photo/audio assets
    ├── peekaboo.db   # SQLite database
    └── age.key       # Encryption key
```

## MVP Animals

The app supports 6 animals with CC0/public domain media:
- Cat, Dog, Duck, Pig, Chicken, Cow

## Testing

```bash
# Backend tests
cd backend && go test ./...

# Frontend unit tests
cd frontend && npm test

# E2E tests
cd frontend && npx playwright test
```

## Documentation

- [Architecture Spec](specs/architecture.md) - System design and components
- [Piper TTS Spec](specs/piper.md) - Future TTS integration
- [Deployment Guide](docs/DEPLOY.md) - Production deployment

## License

Private project. Media assets are CC0/Public Domain.
