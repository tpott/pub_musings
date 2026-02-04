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
- age (for encryption) - `brew install age` on macOS, or see [age releases](https://github.com/FiloSottile/age/releases)
- whisper-server (see [Whisper Server Setup](#whisper-server-setup) below)
- Anthropic or OpenAI API key (see [LLM Configuration](#llm-configuration) below)

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
   # Edit .env with your API keys (see sections below)
   ```

5. Start whisper-server (see [Whisper Server Setup](#whisper-server-setup))

6. Start the backend:
   ```bash
   cd backend && go run main.go
   ```

7. Start the frontend (in another terminal):
   ```bash
   cd frontend && npm run dev
   ```

8. Open http://localhost:4321 in your browser

## LLM Configuration

Peekaboo uses an LLM for intent extraction (understanding "show me a cat" → subject: cat). It supports both Anthropic and OpenAI.

### Using Anthropic (Default)

1. Get an API key from [Anthropic Console](https://console.anthropic.com/)
2. Add to your `.env` file:
   ```bash
   LLM_PROVIDER=anthropic
   ANTHROPIC_API_KEY=sk-ant-api03-xxxxx
   ```

### Using OpenAI

1. Get an API key from [OpenAI Platform](https://platform.openai.com/)
2. Add to your `.env` file:
   ```bash
   LLM_PROVIDER=openai
   OPENAI_API_KEY=sk-xxxxx
   ```

### Models Used

- Anthropic: `claude-3-haiku-20240307` (fast and cost-effective)
- OpenAI: `gpt-4o-mini`

## Whisper Server Setup

Peekaboo forwards audio to a [whisper.cpp](https://github.com/ggml-org/whisper.cpp) server for transcription.

### Building whisper.cpp

```bash
# Clone the repository
git clone https://github.com/ggml-org/whisper.cpp.git
cd whisper.cpp

# Build with CMake
cmake -B build
cmake --build build --config Release

# Download a model (base.en recommended for testing)
./models/download-ggml-model.sh base.en

# For better accuracy in production, use a larger model:
# ./models/download-ggml-model.sh large-v3
```

### Available Models

| Model | Size | English-only | Multilingual | Notes |
|-------|------|--------------|--------------|-------|
| tiny | 75 MB | tiny.en | tiny | Fastest, lowest accuracy |
| base | 142 MB | base.en | base | Good for testing |
| small | 466 MB | small.en | small | Balanced speed/accuracy |
| medium | 1.5 GB | medium.en | medium | Higher accuracy |
| large-v3 | 3.1 GB | - | large-v3 | Best accuracy, slowest |

### Running the Server

```bash
# Start with default settings (port 8765)
./build/bin/whisper-server \
  -m models/ggml-base.en.bin \
  --host 127.0.0.1 \
  --port 8765 \
  -t 4 \
  --convert

# For production with larger model:
./build/bin/whisper-server \
  -m models/ggml-large-v3.bin \
  --host 127.0.0.1 \
  --port 8765 \
  -t 8 \
  --convert
```

Key options:
- `-m MODEL` - Path to model file
- `--host HOST` - Server host (default: 127.0.0.1)
- `--port PORT` - Server port (default: 8080, we use 8765)
- `-t N` - Number of threads (default: 4)
- `--convert` - Enable automatic WAV conversion via ffmpeg

### Testing the Server

```bash
# Test with a sample audio file
curl -X POST http://127.0.0.1:8765/inference \
  -F "file=@tests/fixtures/show-me-cat.webm" \
  -F "temperature=0" \
  -F "response_format=verbose_json" \
  -F "language=en"
```

Expected response:
```json
{
  "task": "transcribe",
  "language": "en",
  "duration": 1.5,
  "text": "Show me a cat."
}
```

### Environment Variable

Configure the whisper server URL in your `.env`:
```bash
WHISPER_SERVER_URL=http://127.0.0.1:8765
```

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

- [API Reference](docs/API.md) - Backend endpoint documentation
- [Architecture Spec](specs/architecture.md) - System design and components
- [Piper TTS Spec](specs/piper.md) - Future TTS integration
- [Deployment Guide](docs/DEPLOY.md) - Production deployment

## License

Private project. Media assets are CC0/Public Domain.
