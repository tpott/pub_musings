# Piper TTS Spec

**Researched:** 2026-02-04
**Confidence:** HIGH

## Overview

Piper is a fast, local neural text-to-speech system optimized for edge devices like Raspberry Pi. It runs entirely offline with no cloud dependencies, making it ideal for privacy-focused applications.

## Installation

### Via pip (Recommended)

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install piper-tts
```

For HTTP server support:

```bash
pip install piper-tts[http]
```

### System Requirements

- Python 3.9+
- Linux (x86_64 or ARM64) or Windows with WSL
- Tested on: Raspberry Pi OS 64-bit, Ubuntu 22.04, Windows 11 WSL

### GPU Acceleration (Optional)

For CUDA support:

```bash
pip install onnxruntime-gpu
```

## Voice Models

Piper uses ONNX-based voice models. Each voice requires:
- `.onnx` model file
- `.json` config file (same name as model)

### Downloading Voices

```bash
python3 -m piper.download_voices en_US-lessac-medium
```

Models are stored in `~/.local/share/piper-voices/` by default.

### Recommended Voices for Peekaboo

| Voice | Quality | Speed | Notes |
|-------|---------|-------|-------|
| `en_US-lessac-medium` | Good | Fast | Clear female voice, good for children |
| `en_US-amy-medium` | Good | Fast | British female voice |
| `en_GB-alan-medium` | Good | Fast | British male voice |

### Voice Quality Levels

- **x-low**: Fastest, lowest quality (~16kHz)
- **low**: Fast, basic quality (~16kHz)
- **medium**: Good balance of speed and quality (~22kHz)
- **high**: Slower, better quality (~22kHz)

For peekaboo, **medium** voices provide the best balance for responsive interaction.

### Multi-Speaker Models

Some models contain multiple speakers. Use the `speaker` or `speaker_id` parameter to select. Note: Individual speaker quality may be lower than single-speaker models.

## HTTP Server Deployment

### Starting the Server

```bash
python3 -m piper.http_server -m en_US-lessac-medium --host 0.0.0.0 --port 5000
```

### API Usage

**Endpoint:** `POST /`

**Request:**

```bash
curl -X POST -H 'Content-Type: application/json' \
  -d '{"text": "Show me a cat!"}' \
  -o output.wav \
  http://localhost:5000
```

**JSON Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `text` | string | Yes | Text to synthesize |
| `voice` | string | No | Voice name (if server loaded multiple) |
| `speaker` | string | No | Speaker name for multi-speaker models |
| `speaker_id` | int | No | Speaker ID (overrides speaker) |
| `length_scale` | float | No | Speaking speed (default: 1.0, lower = faster) |
| `noise_scale` | float | No | Speaking variability |

**Response:** WAV audio file (16-bit PCM)

### Example Integration with Go Backend

```go
func synthesizeSpeech(text string) ([]byte, error) {
    payload := map[string]string{"text": text}
    body, _ := json.Marshal(payload)

    resp, err := http.Post(
        "http://localhost:5000",
        "application/json",
        bytes.NewReader(body),
    )
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}
```

## Docker Deployment

### Using serve-piper-tts (Go wrapper)

```bash
docker pull ghcr.io/arunk140/serve-piper-tts:latest
docker run -p 8080:8080 ghcr.io/arunk140/serve-piper-tts:latest
```

API: `GET /api/tts?text=Hello%20world`

### Custom Dockerfile

```dockerfile
FROM python:3.11-slim

RUN pip install piper-tts[http]
RUN python3 -m piper.download_voices en_US-lessac-medium

EXPOSE 5000
CMD ["python3", "-m", "piper.http_server", "-m", "en_US-lessac-medium", "--host", "0.0.0.0"]
```

## Peekaboo Integration Plan

### Use Case

After displaying animal media, speak the animal name: "This is a cat!"

### Recommended Architecture

```
Frontend                    Backend                     Piper Server
    │                          │                            │
    │  GET /api/media/cat      │                            │
    │─────────────────────────>│                            │
    │                          │  POST {"text": "cat"}      │
    │                          │───────────────────────────>│
    │                          │  <wav audio>               │
    │                          │<───────────────────────────│
    │  {photo, audio, tts_url} │                            │
    │<─────────────────────────│                            │
```

### Configuration

Add to `secrets.enc.yaml`:

```yaml
PIPER_SERVER_URL: http://localhost:5000
```

### MVP vs Future

**MVP (current):** Pre-recorded animal sounds only. No TTS needed.

**Future enhancement:** Add TTS for dynamic responses like "Here's a cat!" or "I don't know that animal."

## Performance Notes

- Synthesis latency: ~50-200ms for short phrases on modern CPUs
- First request may be slower (model loading)
- Memory usage: ~200-500MB per loaded voice
- CPU usage: Moderate during synthesis, idle otherwise

## Sources

- [Piper GitHub](https://github.com/rhasspy/piper)
- [Piper Voice Samples](https://rhasspy.github.io/piper-samples/)
- [piper-tts on PyPI](https://pypi.org/project/piper-tts/)
- [Piper Voices on Hugging Face](https://huggingface.co/rhasspy/piper-voices)
- [serve-piper-tts Go wrapper](https://github.com/arunk140/serve-piper-tts)
- [Piper HTTP Server Docs](https://docs.pipecat.ai/server/services/tts/piper)
