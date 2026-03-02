# Piper TTS Setup

Piper is a fast, local neural text-to-speech engine. Peekaboo uses it to speak animal names and responses aloud ("This is a cat!"). It runs on the Mac Mini host alongside whisper-server.

## Architecture

```
VM (Go backend)                     Mac Mini Host
    │                                   │
    │  POST {"text":"This is a cat!"}   │
    │──────────────────────────────────>│  Piper TTS (:8051)
    │  <wav audio>                      │
    │<──────────────────────────────────│
```

The Go backend reaches Piper at `http://10.0.2.2:8051` from inside the VM (qemu user-mode gateway), or `http://localhost:8051` for local development.

## Installation

### 1. Create Python venv and install

```bash
cd ~/Github/pub_musings/peekaboo
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements-piper.txt
```

### 2. Download voice model

```bash
source .venv/bin/activate
mkdir -p data/piper-voices
python3 -m piper.download_voices en_US-lessac-high \
  --download-dir data/piper-voices
```

This downloads `en_US-lessac-high` (~109MB), a high-quality American English female voice at 22050Hz.

### Available Voices

| Voice | Quality | Size | Notes |
|-------|---------|------|-------|
| `en_US-lessac-high` | High | 109 MB | Default. Clear female voice |
| `en_US-lessac-medium` | Medium | 61 MB | Faster, slightly lower quality |
| `en_US-amy-medium` | Medium | 61 MB | British female voice |
| `en_GB-alan-medium` | Medium | 61 MB | British male voice |

To use a different voice, download it and set `PIPER_VOICE` when starting the server.

## Running

### Manual start

```bash
./scripts/start-piper-server.sh
```

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PIPER_PORT` | `8051` | HTTP server port |
| `PIPER_HOST` | `0.0.0.0` | Server bind address |
| `PIPER_VOICE` | `en_US-lessac-high` | Voice model name |

### Auto-start with launchd (production)

Create `~/Library/LaunchAgents/com.user.piper-server.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.user.piper-server</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/trevor/Github/pub_musings/peekaboo/scripts/start-piper-server.sh</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/Users/trevor/Library/Logs/piper-server.log</string>
    <key>StandardErrorPath</key>
    <string>/Users/trevor/Library/Logs/piper-server.error.log</string>
</dict>
</plist>
```

> **Note:** The `EnvironmentVariables` section is required because launchd does not inherit the user's shell PATH. Without it, `python3` may not be found.

Load and start:

```bash
launchctl load ~/Library/LaunchAgents/com.user.piper-server.plist
launchctl start com.user.piper-server
```

Stop and unload:

```bash
launchctl stop com.user.piper-server
launchctl unload ~/Library/LaunchAgents/com.user.piper-server.plist
```

## Testing

### Check server is running

```bash
curl -s http://localhost:8051/voices | python3 -m json.tool
```

### Synthesize audio

```bash
curl -s -X POST -H 'Content-Type: application/json' \
  -d '{"text":"Hello, this is a test."}' \
  -o /tmp/piper-test.wav http://localhost:8051

# Check file size (should be >10KB for a short phrase)
ls -lh /tmp/piper-test.wav

# Play it (macOS)
afplay /tmp/piper-test.wav
```

### Test from VM

```bash
curl -s -X POST -H 'Content-Type: application/json' \
  -d '{"text":"This is a cat."}' \
  -o /tmp/piper-test.wav http://10.0.2.2:8051
```

## Backend Configuration

Set `PIPER_SERVER_URL` in `.env` or `secrets.enc.yaml`:

```bash
# Local development
PIPER_SERVER_URL=http://localhost:8051

# Production (VM accessing host)
PIPER_SERVER_URL=http://10.0.2.2:8051
```

TTS is optional — the backend works without it. If `PIPER_SERVER_URL` is not set, TTS is disabled.

## Troubleshooting

### Port already in use

macOS AirPlay Receiver uses port 5000. Piper defaults to port 8051 to avoid this. If port 8051 is also in use:

```bash
PIPER_PORT=8052 ./scripts/start-piper-server.sh
```

### Empty or tiny WAV output

If the WAV file is <1KB, the server likely returned an error. Check the server logs:

```bash
tail -f ~/Library/Logs/piper-server.error.log
```

### Server not starting (launchd)

```bash
# Check if loaded
launchctl list | grep piper

# Check logs
tail -20 ~/Library/Logs/piper-server.error.log

# Common fix: PATH not set, python3 not found
# Ensure EnvironmentVariables.PATH includes /opt/homebrew/bin
```

### Voice model not found

The server expects `data/piper-voices/<voice>.onnx` and `<voice>.onnx.json`. Re-download:

```bash
source .venv/bin/activate
python3 -m piper.download_voices en_US-lessac-high \
  --download-dir data/piper-voices --force-redownload
```

## API Reference

**Endpoint:** `POST /`

```json
{
  "text": "This is a cat!",
  "length_scale": 1.0,
  "noise_scale": 0.667
}
```

**Response:** WAV audio (16-bit PCM, mono, 22050Hz)

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `text` | string | Yes | Text to synthesize |
| `length_scale` | float | No | Speaking speed (lower = faster, default: 1.0) |
| `noise_scale` | float | No | Speaking variability (default: 0.667) |

**Other endpoints:**
- `GET /voices` — List loaded voice models
