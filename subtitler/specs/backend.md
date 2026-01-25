# Backend Spec

## Overview

The subtitler backend is written in Go. It handles API requests from the frontend and uses whisper.cpp for transcription.

## Whisper.cpp Setup (Mac Mini)

### Clone the Repository

```bash
cd ~/Github
git clone https://github.com/ggml-org/whisper.cpp.git
cd whisper.cpp
```

### Build Options

Metal GPU acceleration is enabled by default on Apple Silicon.

**Basic build:**
```bash
cmake -B build
cmake --build build -j --config Release
```

**Static linking (recommended):**
```bash
cmake -B build -DBUILD_SHARED_LIBS=OFF
cmake --build build -j --config Release
```

**With SDL2 support (for microphone input):**
```bash
cmake -B build -DBUILD_SHARED_LIBS=OFF -DWHISPER_SDL2=ON
cmake --build build -j --config Release
```

### Common CMake Flags

| Flag | Description |
|------|-------------|
| `-DBUILD_SHARED_LIBS=OFF` | Static linking (recommended) |
| `-DWHISPER_SDL2=ON` | Enable SDL2 for microphone/audio input |
| `-DWHISPER_COREML=ON` | Enable Core ML acceleration |
| `-DWHISPER_METAL=ON` | Enable Metal GPU (default on macOS) |

### Verify the Build

```bash
./build/bin/whisper-cli -f samples/jfk.wav
```

### Download Models

```bash
# Download models (choose based on speed vs accuracy needs)
sh ./models/download-ggml-model.sh base.en        # Fast, English only
sh ./models/download-ggml-model.sh large-v3-turbo # Accurate, multilingual

# Download voice activity detection model (Silero VAD)
# Check https://github.com/ggml-org/whisper.cpp for latest VAD model URL
```

Available models: `tiny.en`, `tiny`, `base.en`, `base`, `small.en`, `small`, `medium.en`, `medium`, `large-v1`, `large-v2`, `large-v3`, `large-v3-turbo`

### Model Locations

After downloading, models are stored at:
```
~/Github/whisper.cpp/models/ggml-large-v3-turbo.bin
~/Github/whisper.cpp/models/ggml-silero-v6.2.0.bin
```

## Whisper Server

### Build the Server

The server is built as part of the main build:

```bash
cd ~/Github/whisper.cpp
cmake -B build -DBUILD_SHARED_LIBS=OFF
cmake --build build -j --config Release
```

The server binary is at `./build/bin/whisper-server`.

### Run the Server

```bash
./build/bin/whisper-server \
    -m ./models/ggml-large-v3-turbo.bin \
    --host 0.0.0.0 \
    --port 8050
```

### Server Options

| Flag | Description |
|------|-------------|
| `-m, --model` | Path to the whisper model file |
| `--host` | Host to bind to (default: 127.0.0.1) |
| `--port` | Port to listen on (default: 8080) |
| `-t, --threads` | Number of threads to use |
| `-l, --language` | Language code (e.g., `en`, `es`, `auto`) |

### API Endpoints

- `POST /inference` - Transcribe audio file
- `GET /load` - Load a model
- `GET /health` - Health check

### Example Request

```bash
curl -X POST http://localhost:8050/inference \
    -F "file=@audio.wav" \
    -F "response_format=json"
```

### Run as launchd Service (Mac Mini)

Create `~/Library/LaunchAgents/whisper-server.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>whisper-server</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/trevor/Github/whisper.cpp/build/bin/whisper-server</string>
        <string>-m</string>
        <string>/Users/trevor/Github/whisper.cpp/models/ggml-large-v3-turbo.bin</string>
        <string>--host</string>
        <string>0.0.0.0</string>
        <string>--port</string>
        <string>8050</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/Users/trevor/Github/whisper.cpp</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/whisper-server.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/whisper-server.err</string>
</dict>
</plist>
```

Load and start the service:

```bash
launchctl load ~/Library/LaunchAgents/whisper-server.plist
```

Stop and unload:

```bash
launchctl unload ~/Library/LaunchAgents/whisper-server.plist
```

Check status:

```bash
launchctl list | grep whisper
tail -f /tmp/whisper-server.log
```

## Running whisper-cli Directly

For local testing without the server:

```bash
whisper-cli \
    -m ~/Github/whisper.cpp/models/ggml-large-v3-turbo.bin \
    -vm ~/Github/whisper.cpp/models/ggml-silero-v6.2.0.bin \
    -f "input.wav"
```

See `cc_plugins/skills/transcribe-srt/SKILL.md` for converting output to SRT format.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `WHISPER_SERVER_URL` | URL of remote whisper-server (e.g., `http://192.168.1.100:8050`) |
| `WHISPER_MODEL` | Model to use (default: `large-v3-turbo`) |

## Core ML Optimization (Optional)

For additional performance on Apple Silicon:

```bash
# Install dependencies
pip install ane_transformers openai-whisper coremltools

# Generate Core ML model
./models/generate-coreml-model.sh large-v3-turbo

# Rebuild with Core ML support
cmake -B build -DBUILD_SHARED_LIBS=OFF -DWHISPER_COREML=ON
cmake --build build -j --config Release
```

Note: First run with Core ML is slow due to compilation, subsequent runs are faster.
