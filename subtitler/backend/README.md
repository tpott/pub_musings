# Subtitler Backend

Go HTTP server for the Subtitler application. Handles video uploads, transcription via Whisper AI, subtitle generation, and user authentication.

## Quick Start

```bash
# Run the server
/home/trevor/go/bin/go run main.go
# Server runs on http://localhost:8080
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `WHISPER_MODEL` | `$HOME/Github/whisper.cpp/models/ggml-medium.bin` | Path to whisper model (CLI mode only) |
| `WHISPER_SERVER_URL` | `http://127.0.0.1:8765` | URL of whisper-server (enables server mode) |
| `USE_WHISPER_SERVER` | `false` | Set to `true` to use whisper-server with default URL |

### Whisper Mode Selection

The backend supports two modes for speech-to-text transcription:

**CLI Mode (default):**
- Spawns `whisper-cli` subprocess for each transcription
- Model loaded from disk each time (slower)
- Requires whisper-cli in PATH and model file available

**Server Mode:**
- Sends audio to whisper-server HTTP API
- Model stays loaded in memory (faster)
- Supports remote transcription (GPU server)

Server mode is enabled when:
1. `WHISPER_SERVER_URL` is set to any URL, OR
2. `USE_WHISPER_SERVER=true` (uses default URL)

Example:
```bash
# CLI mode with custom model
export WHISPER_MODEL="/path/to/ggml-large-v3.bin"

# Server mode with default localhost URL
export USE_WHISPER_SERVER=true

# Server mode with remote server
export WHISPER_SERVER_URL="http://192.168.1.100:8765"
```

## Running whisper-server

Start whisper-server on the same machine or a separate GPU server:

```bash
cd ~/Github/whisper.cpp

# Build (if not already built)
cmake -B build && cmake --build build --config Release -j

# Run server
./build/bin/whisper-server \
  -m models/ggml-medium.bin \
  --host 0.0.0.0 \
  --port 8765 \
  --convert
```

Options:
- `-m`: Model file path
- `--host`: Bind address (`0.0.0.0` for external access, `127.0.0.1` for local only)
- `--port`: Port number (use 8765 to avoid conflict with backend's 8080)
- `--convert`: Auto-convert audio formats via ffmpeg
- `-t N`: Number of threads (default: 4)

## qemu VM Configuration

When running the backend inside a qemu VM with whisper-server on the host, configure networking to allow the VM to reach the host.

### Option 1: User-mode networking with hostfwd (Recommended)

This is the simplest setup - the VM can access the host via the gateway IP.

```bash
# Start qemu with user networking (default)
qemu-system-x86_64 \
  -netdev user,id=net0 \
  -device e1000,netdev=net0 \
  ...
```

Inside the VM, the host is accessible at `10.0.2.2` (default qemu gateway):

```bash
# In the VM
export WHISPER_SERVER_URL="http://10.0.2.2:8765"
```

### Option 2: Bridge networking

For more advanced networking, create a bridge on the host:

```bash
# On host: Create bridge (one-time setup)
sudo ip link add br0 type bridge
sudo ip link set br0 up
sudo ip addr add 192.168.100.1/24 dev br0

# Start qemu with bridge
qemu-system-x86_64 \
  -netdev bridge,id=net0,br=br0 \
  -device virtio-net,netdev=net0 \
  ...
```

Configure VM network:
```bash
# In the VM
sudo ip addr add 192.168.100.2/24 dev eth0
export WHISPER_SERVER_URL="http://192.168.100.1:8765"
```

### Option 3: macvtap (Direct host NIC access)

For production deployments where the VM needs its own IP on the LAN:

```bash
qemu-system-x86_64 \
  -netdev tap,id=net0,ifname=macvtap0,script=no,downscript=no \
  -device virtio-net,netdev=net0 \
  ...
```

The VM gets a LAN IP via DHCP. Configure `WHISPER_SERVER_URL` with the host's LAN IP.

### Firewall Configuration

Ensure the host firewall allows connections to whisper-server:

```bash
# Allow port 8765 from VM network
sudo ufw allow from 10.0.2.0/24 to any port 8765
# Or for bridge network
sudo ufw allow from 192.168.100.0/24 to any port 8765
```

### Verifying Connectivity

From inside the VM:

```bash
# Test whisper-server health endpoint
curl http://10.0.2.2:8765/health
# Expected: {"status":"ok"}

# Test inference (requires a WAV file)
curl http://10.0.2.2:8765/inference \
  -F file=@test.wav \
  -F response_format=json
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Health check |
| POST | `/api/upload` | Upload video file |
| POST | `/api/transcribe/{id}` | Start transcription |
| GET | `/api/transcribe/{id}` | Get transcription status |
| PUT | `/api/transcribe/{id}/segments` | Update subtitle segments |
| POST | `/api/transcribe/{id}/align` | Align pasted transcript |
| GET | `/api/videos` | List user's videos |
| GET | `/api/videos/{id}/video` | Stream video file |
| GET | `/api/videos/{id}/subtitles.srt` | Download SRT file |
| POST | `/api/videos/{id}/burn` | Start subtitle burning |
| GET | `/api/videos/{id}/burn` | Get burn job status |
| GET | `/api/videos/{id}/burned` | Download burned video |
| POST | `/api/auth/register` | Create account |
| POST | `/api/auth/login` | Login |
| POST | `/api/auth/logout` | Logout |
| GET | `/api/auth/me` | Get current user |
| POST | `/api/auth/totp/setup` | Setup 2FA |
| POST | `/api/auth/totp/verify` | Verify 2FA code |
| POST | `/api/auth/totp/disable` | Disable 2FA |
| POST | `/api/log` | Frontend console log forwarding |

## Project Structure

```
backend/
├── main.go          # HTTP handlers and server setup
├── main_test.go     # SRT formatting tests
├── api_test.go      # API integration tests
├── align/           # Transcript alignment algorithm
├── auth/            # Authentication and sessions
├── crypto/          # File encryption (age)
├── db/              # SQLite database layer
└── totp/            # TOTP 2FA implementation
```

## Testing

```bash
# Run all backend tests
/home/trevor/go/bin/go test ./... -v

# Run with coverage
/home/trevor/go/bin/go test ./... -cover

# Run specific package tests
/home/trevor/go/bin/go test ./auth -v
```

## See Also

- [../README.md](../README.md) - Project overview
- [../INSTALL.md](../INSTALL.md) - Installation guide
- [../TESTING.md](../TESTING.md) - Test documentation
- [../specs/auth.md](../specs/auth.md) - Auth specification
- [../specs/encryption.md](../specs/encryption.md) - Encryption specification
- [../specs/totp.md](../specs/totp.md) - TOTP specification
