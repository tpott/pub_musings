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

# Add to PATH (add to ~/.bashrc or ~/.zshrc)
export PATH=$PATH:/usr/local/go/bin
```

**macOS (Homebrew):**
```bash
brew install go
```

**Note:** If Go is not in your PATH, use the full path: `/home/trevor/go/bin/go`

**Verify installation:**
```bash
go version  # Should be go1.22+
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

## Optional Dependencies

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

The backend supports the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend server port |
| `WHISPER_MODEL` | `$HOME/Github/whisper.cpp/models/ggml-medium.bin` | Path to whisper model file |

**Example configuration:**
```bash
# Add to ~/.bashrc or ~/.zshrc
export WHISPER_MODEL="$HOME/Github/whisper.cpp/models/ggml-large-v3-turbo.bin"
```

## Quick Start

After installing all dependencies:

```bash
# Start backend (terminal 1)
cd backend
go run main.go
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
Use the full path to Go if it's not in PATH:
```bash
/home/trevor/go/bin/go run main.go
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
