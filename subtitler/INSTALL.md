# Installation Guide

This guide covers installing all dependencies required to run the Subtitler application.

## Prerequisites

- Linux or macOS
- A terminal with sudo access (for some installations)

## Required Dependencies

### 1. Node.js (v18+)

The frontend uses Astro which requires Node.js 18 or later.

**Ubuntu/Debian:**
```bash
# Using NodeSource repository
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs
```

**macOS (Homebrew):**
```bash
brew install node
```

**Verify installation:**
```bash
node --version  # Should be v18+
npm --version
```

### 2. Go (v1.22+)

The backend is written in Go and requires version 1.22 or later.

**Ubuntu/Debian:**
```bash
# Download and install
wget https://go.dev/dl/go1.22.10.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.22.10.linux-amd64.tar.gz
```

**macOS (Homebrew):**
```bash
brew install go
```

**IMPORTANT: Add Go to PATH**

Go must be in your PATH for scripts and automated tools (like Claude Code) to work. Add this to `~/.profile` (not just `~/.bashrc`, since automated tools may not source `.bashrc`):

```bash
# Add to ~/.profile for all shells including non-interactive
export PATH="$PATH:/usr/local/go/bin"
export PATH="$PATH:$HOME/go/bin"  # For go install'd binaries
```

Then reload:
```bash
source ~/.profile
```

**Why `~/.profile` instead of `~/.bashrc`?**
- `~/.bashrc` is only sourced by interactive Bash shells
- `~/.profile` is sourced by login shells and many automated tools
- Automated systems (CI, Claude Code, cron) often run non-interactive shells that skip `.bashrc`

**Verify installation:**
```bash
go version  # Should be go1.22+
which go    # Should show the Go binary path
```

### 3. FFmpeg

FFmpeg is used for audio extraction and subtitle burning.

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install -y ffmpeg
```

**macOS (Homebrew):**
```bash
brew install ffmpeg
```

**Verify installation:**
```bash
ffmpeg -version
```

### 4. whisper-cli (from whisper.cpp)

The application uses whisper-cli for speech-to-text transcription.

**Build from source:**
```bash
# Clone the repository
cd ~/Github  # or your preferred directory
git clone https://github.com/ggml-org/whisper.cpp.git
cd whisper.cpp

# Build whisper-cli
make

# Add to PATH or create symlink
sudo ln -s $(pwd)/build/bin/whisper-cli /usr/local/bin/whisper-cli
```

**Download a model:**
```bash
# Download the medium model (recommended for balance of speed/accuracy)
cd ~/Github/whisper.cpp
./models/download-ggml-model.sh medium

# Or download other models:
# ./models/download-ggml-model.sh tiny     # Fastest, least accurate
# ./models/download-ggml-model.sh base
# ./models/download-ggml-model.sh small
# ./models/download-ggml-model.sh large-v3 # Most accurate, slowest
# ./models/download-ggml-model.sh large-v3-turbo  # Good balance
```

**Verify installation:**
```bash
whisper-cli --help
ls ~/Github/whisper.cpp/models/  # Should show ggml-*.bin files
```

### 5. whisper-server (Alternative to whisper-cli)

For better performance, you can run whisper-server instead of spawning whisper-cli for each transcription. The server keeps the model loaded in memory.

**Build and run whisper-server:**
```bash
cd ~/Github/whisper.cpp

# Build with server support
cmake -B build
cmake --build build --config Release -j

# Run the server (example with medium model)
./build/bin/whisper-server \
  -m models/ggml-medium.bin \
  --host 127.0.0.1 \
  --port 8765 \
  --convert  # Auto-convert audio formats via ffmpeg
```

**Server options:**
- `--host`: Bind address (default: 127.0.0.1)
- `--port`: Port number (default: 8080, but use 8765 to avoid conflict with backend)
- `--convert`: Enable automatic audio format conversion via ffmpeg
- `-t N`: Number of threads (default: 4)
- `-l LANG`: Default language (use 'auto' for auto-detect)

**Verify server is running:**
```bash
curl http://127.0.0.1:8765/health
# Expected: {"status":"ok"}
```

## Optional Dependencies

### Fonts for Indic Script Support (Hindi, Tamil, Telugu, etc.)

When burning subtitles into video, FFmpeg needs fonts that support the character set being used. For Indic scripts (Devanagari, Tamil, Telugu, etc.), you need to install appropriate fonts.

**Ubuntu/Debian:**
```bash
# Install Noto fonts (comprehensive Unicode coverage)
sudo apt-get install -y fonts-noto fonts-noto-cjk fonts-noto-extra

# Or install specific Indic fonts
sudo apt-get install -y fonts-noto-core fonts-indic
```

**macOS:**
- Noto fonts are not included by default
- Download from https://fonts.google.com/noto
- Or install via Homebrew: `brew install font-noto-sans-devanagari` (requires `brew tap homebrew/cask-fonts`)

**Configure the subtitle font:**
Set the `SUBTITLE_FONT` environment variable to a font that supports your target scripts:

```bash
# Use a specific font file
export SUBTITLE_FONT="/usr/share/fonts/truetype/noto/NotoSansDevanagari-Regular.ttf"

# Or use a font name (fontconfig will resolve it)
export SUBTITLE_FONT="Noto Sans Devanagari"
```

**Verify fonts are installed:**
```bash
# List fonts supporting Hindi
fc-list :lang=hi

# List all Noto fonts
fc-list | grep -i noto
```

**Why this matters:**
- Without proper fonts, non-Latin characters appear as empty boxes (□) in burned subtitles
- The downloadable SRT/VTT files are not affected (they contain the text correctly)
- Only the "burn into video" feature requires the fonts

### SQLite3 (usually pre-installed)

The backend uses SQLite for data storage. It's typically pre-installed on most systems.

**Ubuntu/Debian (if needed):**
```bash
sudo apt-get install -y sqlite3 libsqlite3-dev
```

**Verify:**
```bash
sqlite3 --version
```

## Environment Variables

The backend supports many environment variables for configuration. Here are the key ones for installation:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend server port |
| `WHISPER_MODEL` | `$HOME/Github/whisper.cpp/models/ggml-medium.bin` | Path to whisper model file (CLI mode only) |
| `WHISPER_SERVER_URL` | `http://127.0.0.1:8765` | URL of whisper-server (enables server mode when set) |
| `USE_WHISPER_SERVER` | `false` | Set to `true` to use whisper-server even without custom URL |

> **Complete reference:** For all environment variables including rate limits, email settings, security options, and debugging configuration, see [docs/ENV.md](docs/ENV.md).

**Whisper Mode Selection:**
- **CLI mode (default):** Uses `whisper-cli` spawned as subprocess. Simple but slower (model loaded each time).
- **Server mode:** Uses HTTP API to whisper-server. Faster (model stays in memory).

Server mode is enabled when:
1. `WHISPER_SERVER_URL` is set (to any URL), OR
2. `USE_WHISPER_SERVER=true` (uses default URL `http://127.0.0.1:8765`)

**Example configurations:**
```bash
# CLI mode with custom model
export WHISPER_MODEL="$HOME/Github/whisper.cpp/models/ggml-large-v3-turbo.bin"

# Server mode with default URL
export USE_WHISPER_SERVER=true

# Server mode with custom URL (e.g., remote server)
export WHISPER_SERVER_URL="http://192.168.1.100:8765"
```

## Quick Start

After installing all dependencies:

```bash
# Start backend (terminal 1)
cd backend
go run .
# Runs on http://localhost:8080

# Start frontend (terminal 2)
cd frontend
npm install
npm run dev
# Runs on http://localhost:4321
```

**Verify setup:**
```bash
# Check backend health
curl http://localhost:8080/api/health
# Expected: {"status":"ok"}

# Check frontend
curl http://localhost:4321
# Expected: HTML page
```

## Troubleshooting

### "whisper model not found" error
Ensure the `WHISPER_MODEL` environment variable points to a valid model file, or that the default path exists:
```bash
ls $HOME/Github/whisper.cpp/models/ggml-medium.bin
```

### "go: command not found"
Go is not in your PATH. Follow the "IMPORTANT: Add Go to PATH" instructions in the Go section above. Make sure to add the exports to `~/.profile` and run `source ~/.profile`.

Verify with:
```bash
which go
go version
```

### FFmpeg errors
Ensure FFmpeg is installed and in PATH:
```bash
which ffmpeg
ffmpeg -version
```

### CGO errors when building Go
The sqlite3 driver requires CGO. Ensure you have a C compiler:
```bash
# Ubuntu/Debian
sudo apt-get install -y build-essential
```

## See Also

- [README.md](README.md) - Project overview and quick start
- [TESTING.md](TESTING.md) - Running tests
- [specs/subtitler.md](specs/subtitler.md) - Project specification
