# Troubleshooting Guide

Common issues and their solutions when running Peekaboo.

## Microphone Access Issues

### Microphone not working / button disabled

**Symptoms:**
- Mic button is disabled or grayed out
- No audio is recorded when pressing the button
- Browser shows no microphone permission prompt

**Diagnosis:**

```bash
# Check if running over HTTPS (required for mic access in production)
curl -v https://your-domain.com/health 2>&1 | grep -E "SSL|TLS"
```

**Solutions:**

1. **HTTPS required in production**: Browsers only allow microphone access on secure contexts (HTTPS or localhost). If running over plain HTTP, the mic will silently fail.

2. **Check browser permissions**: Look for a camera/microphone icon in the browser address bar. Click it to grant or verify permissions.

3. **Try a different browser**: Some browsers have stricter security policies. Chrome and Firefox are well-tested.

4. **Check for MediaRecorder support**:
   ```javascript
   // Run in browser console
   console.log('MediaRecorder supported:', typeof MediaRecorder !== 'undefined');
   console.log('getUserMedia supported:', !!navigator.mediaDevices?.getUserMedia);
   ```

5. **Check browser console for errors**: Open DevTools (F12) and look for errors like:
   - `NotAllowedError` - user denied permission
   - `NotFoundError` - no microphone detected
   - `NotReadableError` - microphone in use by another app

### Microphone permission denied

**Symptoms:**
- Browser console shows `NotAllowedError: Permission denied`
- Mic button clicks but nothing happens

**Solutions:**

1. **Reset site permissions in Chrome**:
   - Click padlock icon in address bar → Site settings → Microphone → Allow

2. **Reset site permissions in Firefox**:
   - Click padlock icon → Connection secure → More information → Permissions → Use the Microphone → Allow

3. **Check system-level permissions** (macOS):
   - System Preferences → Security & Privacy → Privacy → Microphone
   - Ensure your browser is checked

---

## Whisper Server Connectivity

### Health check shows whisper unavailable

**Symptoms:**
- `/health/ready` returns 503 with `"whisper": "unavailable"`
- Transcription requests fail with 500 error

**Diagnosis:**

```bash
# Check health endpoint
curl http://localhost:8080/health/ready
# Expected: {"status":"ok","database":"ok","whisper":"ok"}

# Check whisper-server directly
curl http://127.0.0.1:8765/health
# Expected: {"status":"ok"}
```

**Solutions:**

1. **Verify whisper-server is running**:
   ```bash
   # Check if process is running
   pgrep -f whisper-server

   # Check if port is listening
   lsof -i :8765
   ```

2. **Start whisper-server**:
   ```bash
   ./build/bin/whisper-server \
     -m models/ggml-base.en.bin \
     --host 127.0.0.1 \
     --port 8765 \
     -t 4 \
     --convert
   ```

3. **Check WHISPER_SERVER_URL in .env**:
   ```bash
   grep WHISPER_SERVER_URL .env
   # Should be: WHISPER_SERVER_URL=http://127.0.0.1:8765
   ```

4. **For VM deployments**, use the host IP (not localhost):
   ```bash
   # QEMU default gateway
   WHISPER_SERVER_URL=http://10.0.2.2:8765
   ```

5. **For whisper-server on a different host**, use an SSH tunnel.

   If your VM runs on one machine but whisper-server runs on another, create an SSH tunnel on the VM host to forward the port:

   ```bash
   # On VM host: forward local port 8765 to whisper-server host
   ssh -L 0.0.0.0:8765:localhost:8765 whisper-host -N
   ```

   **Flags:**
   - `-L 0.0.0.0:8765:localhost:8765` — Listen on all interfaces on local machine, forward to `localhost:8765` on remote
   - `-N` — No remote command (tunnel only, no shell)

   The VM can then reach whisper-server via the QEMU gateway:
   ```bash
   WHISPER_SERVER_URL=http://10.0.2.2:8765
   ```

   **SSH config alternative** (`~/.ssh/config`):
   ```
   Host whisper-tunnel
       HostName whisper-host
       LocalForward 0.0.0.0:8765 localhost:8765
       ExitOnForwardFailure yes
       ServerAliveInterval 30
       ServerAliveCountMax 3
   ```

   Then run: `ssh -N whisper-tunnel`

### Transcription returns empty text / "No speech detected"

**Symptoms:**
- API returns `{"text": ""}` even with clear audio
- Frontend shows "No speech detected. Please try again."
- No errors in server logs

**Most Common Cause: Missing `--convert` flag**

The browser records audio in webm/opus format, which whisper-server cannot process natively. The `--convert` flag enables ffmpeg conversion.

```bash
# WRONG - will return empty text for webm/opus audio
./whisper-server -m models/ggml-base.en.bin

# CORRECT - enables format conversion
./whisper-server -m models/ggml-base.en.bin --convert
```

**Quick test to verify:**
```bash
# Test with the browser-format audio fixture
curl -X POST http://localhost:8080/api/transcribe \
  -F "audio=@tests/fixtures/me-show-me-a-cat.webm"

# If you get {"text": ""}, whisper-server needs --convert flag
# If you get {"text": "Show me a cat"}, it's working
```

**Other Solutions:**

1. **Check audio quality**: Very short recordings (< 1 second) or very quiet audio may produce empty transcripts.

2. **Try a larger model**: The tiny model may miss quiet or unclear speech:
   ```bash
   ./models/download-ggml-model.sh base.en
   ```

3. **Verify ffmpeg is installed** (required for `--convert`):
   ```bash
   which ffmpeg
   # Should output path like /usr/bin/ffmpeg
   ```

---

## Piper TTS (Text-to-Speech)

> **Note:** TTS is optional. If `PIPER_SERVER_URL` is not set, Peekaboo works without speech synthesis - it just won't announce subjects like "Here is a cat."

### TTS endpoint returns 404

**Symptoms:**
- `/api/speak` returns 404 Not Found
- Frontend silently skips TTS (expected behavior)

**Cause:**
This is normal when TTS is not configured. The frontend gracefully handles this.

**If you want TTS:**
1. Set `PIPER_SERVER_URL` in your `.env`:
   ```bash
   PIPER_SERVER_URL=http://localhost:5000
   ```
2. Restart the backend

### Health check shows piper unavailable

**Symptoms:**
- `/health/ready` returns 503 with `"piper": "unavailable"`
- TTS requests fail

**Diagnosis:**

```bash
# Check health endpoint
curl http://localhost:8080/health/ready
# If PIPER_SERVER_URL is set, expect: {"status":"ok","database":"ok","whisper":"ok","piper":"ok"}
# If PIPER_SERVER_URL is NOT set, piper won't appear in response

# Check piper-server directly
curl http://localhost:5000/
# Expected: responds with info or accepts POST
```

**Solutions:**

1. **Verify piper-server is running**:
   ```bash
   # Check if process is running
   pgrep -f piper

   # Check if port is listening
   lsof -i :5000
   ```

2. **Start piper-server** (if installed):
   ```bash
   # Example with piper HTTP server
   piper --http-port 5000
   ```

3. **Check PIPER_SERVER_URL in .env**:
   ```bash
   grep PIPER_SERVER_URL .env
   # Should be: PIPER_SERVER_URL=http://localhost:5000
   ```

4. **For VM deployments**, use the host IP (not localhost):
   ```bash
   PIPER_SERVER_URL=http://10.0.2.2:5000
   ```

### TTS returns empty or garbled audio

**Symptoms:**
- API returns 200 but audio doesn't play
- Audio sounds corrupted

**Solutions:**

1. **Check text input**: Very short or empty text may produce no audio.

2. **Verify audio format**: Peekaboo expects WAV audio from Piper. Check browser console for decode errors.

3. **Test directly**:
   ```bash
   curl -X POST http://localhost:8080/api/speak \
     -H "Content-Type: application/json" \
     -d '{"text": "Hello world"}' \
     --output test.wav
   file test.wav
   # Should say: RIFF (little-endian) data, WAVE audio
   ```

### TTS rate limited

**Symptoms:**
- API returns 429 Too Many Requests

**Solutions:**
Rate limit is 10 requests per minute per IP (same as other expensive endpoints). Wait 60 seconds or check the `Retry-After` header.

---

## Database Issues

### Database file locked

**Symptoms:**
- Error: `database is locked`
- Operations hang or timeout

**Diagnosis:**

```bash
# Check for processes holding the database
lsof data/peekaboo.db
```

**Solutions:**

1. **Stop other processes using the database**: Only one process should write to SQLite at a time.

2. **Check for stale lock files**:
   ```bash
   ls -la data/peekaboo.db*
   # WAL mode creates .db-wal and .db-shm files
   ```

3. **Delete WAL files if database is corrupted** (data loss risk):
   ```bash
   # Only if database won't open at all
   rm data/peekaboo.db-wal data/peekaboo.db-shm
   ```

### Concept not found

**Symptoms:**
- API returns `{"error": "concept not found"}`
- Media works for some animals but not others

**Diagnosis:**

```bash
# List concepts in database
sqlite3 data/peekaboo.db "SELECT * FROM concepts;"

# Check media sets
sqlite3 data/peekaboo.db "SELECT * FROM media_sets;"
```

**Solutions:**

1. **Re-run source-media.sh** to download media:
   ```bash
   ./scripts/source-media.sh
   ```

2. **Re-initialize database**: Delete and restart the server:
   ```bash
   rm data/peekaboo.db
   cd backend && go run main.go
   ```

### Database path not found

**Symptoms:**
- Error: `no such file or directory`
- Server fails to start

**Solutions:**

1. **Create the data directory**:
   ```bash
   mkdir -p data
   ```

2. **Check DB_PATH in .env**:
   ```bash
   grep DB_PATH .env
   # Default: DB_PATH=data/peekaboo.db
   ```

---

## LLM API Issues

### Intent extraction fails

**Symptoms:**
- API returns `{"error": "intent extraction failed"}`
- Works for transcription but not intent

**Diagnosis:**

```bash
# Test intent endpoint directly
curl -X POST http://localhost:8080/api/intent \
  -H "Content-Type: application/json" \
  -d '{"text": "show me a cat"}'
```

**Solutions:**

1. **Check API key is set**:
   ```bash
   grep -E "(ANTHROPIC|OPENAI)_API_KEY" .env | head -1
   # Should show the key (not empty)
   ```

2. **Verify API key format**:
   - Anthropic keys start with `sk-ant-`
   - OpenAI keys start with `sk-`

3. **Check LLM_PROVIDER setting**:
   ```bash
   grep LLM_PROVIDER .env
   # Should be: LLM_PROVIDER=anthropic or LLM_PROVIDER=openai
   ```

4. **Check API quota/billing**: Visit your provider's dashboard to verify you have remaining credits.

### Rate limit exceeded

**Symptoms:**
- API returns 429 with `{"error": "rate limit exceeded, try again later"}`

**Solutions:**

1. **Wait 60 seconds**: The rate limit resets every minute.

2. **Check the Retry-After header**:
   ```bash
   curl -v -X POST http://localhost:8080/api/intent \
     -H "Content-Type: application/json" \
     -d '{"text": "test"}'
   # Look for: Retry-After: 60
   ```

3. **Rate limit is 10 requests per minute per IP** on `/api/transcribe` and `/api/intent`.

---

## WebSocket Mode Issues

### WebSocket mode not working

**Symptoms:**
- Expected continuous audio streaming but getting HTTP-style behavior
- "No speech detected" error instead of WebSocket-specific errors

**Cause:**
WebSocket mode is enabled by default. It can be disabled via URL parameter.

**Solutions:**

1. **Ensure WebSocket mode is not disabled** via URL parameter:
   ```
   https://peekaboo.example.com/  # WebSocket enabled (default)
   https://peekaboo.example.com/?useWebSocket=false  # HTTP mode
   ```

2. **Or check programmatically** in browser console:
   ```javascript
   // This should NOT be set to false
   console.log((window as any).__PEEKABOO_USE_WEBSOCKET__);
   ```

### WebSocket 403 Forbidden (CORS/Origin Error)

**Symptoms:**
- WebSocket connection rejected with HTTP 403
- Browser console shows "WebSocket connection failed"
- Works on localhost but not in production

**Cause:**
The backend validates the `Origin` header against `ALLOWED_ORIGIN`. If they don't match, the connection is rejected.

**Diagnosis:**
```bash
# Check configured allowed origin
grep ALLOWED_ORIGIN .env

# Test with curl (note: browsers send Origin automatically)
curl -v -H "Origin: https://example.com" \
  -H "Connection: Upgrade" -H "Upgrade: websocket" \
  http://localhost:8080/ws/audio
# 403 means origin mismatch
```

**Solutions:**

1. **Set ALLOWED_ORIGIN to match your frontend domain**:
   ```bash
   # In .env
   ALLOWED_ORIGIN=https://peekaboo.pottingers.us
   ```

2. **For development, use wildcard** (not recommended for production):
   ```bash
   ALLOWED_ORIGIN=*
   ```

3. **Include the protocol**: `https://example.com` not just `example.com`.

4. **Restart backend** after changing `.env`:
   ```bash
   systemctl --user restart peekaboo
   ```

### WebSocket 429 Too Many Requests (Rate Limited)

**Symptoms:**
- WebSocket upgrade fails with HTTP 429
- Error message: "rate limit exceeded, try again later"
- Works initially but fails after several quick attempts

**Cause:**
WebSocket connections are rate limited to 10 new connections per minute per IP address. This prevents abuse but can be hit during rapid testing.

**Diagnosis:**
```bash
# Check for Retry-After header
curl -v -H "Connection: Upgrade" -H "Upgrade: websocket" \
  http://localhost:8080/ws/audio 2>&1 | grep -E "429|Retry-After"
```

**Solutions:**

1. **Wait for rate limit window to reset** (60 seconds):
   ```bash
   # Check Retry-After header for exact wait time
   ```

2. **In development, restart the backend** to reset rate limiter state.

3. **Consider if your client is reconnecting too aggressively**:
   - WebSocket reconnection should use exponential backoff
   - Don't create new connections on every user action

4. **Rate limits are per-IP**: Multiple users behind NAT may share limits.

### WebSocket 503 Service Unavailable (Connection Limit)

**Symptoms:**
- WebSocket upgrade fails with HTTP 503
- Error message: "server at capacity, try again later"
- Works for first N connections but then fails

**Cause:**
The backend limits concurrent WebSocket connections to prevent resource exhaustion. Default limit is 100 connections (configurable via `WEBSOCKET_MAX_CONNECTIONS`).

**Diagnosis:**
```bash
# Check current limit
grep WEBSOCKET_MAX_CONNECTIONS .env

# Count active connections (rough estimate)
netstat -an | grep :8080 | grep ESTABLISHED | wc -l
```

**Solutions:**

1. **Increase connection limit** if server has resources:
   ```bash
   # In .env
   WEBSOCKET_MAX_CONNECTIONS=200
   ```

2. **Check for connection leaks**: Clients should properly close WebSocket connections when done.

3. **Monitor server resources**:
   ```bash
   # Check memory usage
   free -h

   # Check open file descriptors
   cat /proc/$(pgrep peekaboo)/fd | wc -l
   ```

4. **Restart backend** to reset connection count (not recommended in production, investigate root cause first).

### WebSocket connection drops mid-recording

**Symptoms:**
- Recording starts but WebSocket closes unexpectedly
- Error message about connection lost
- Happens after a period of silence

**Cause:**
WebSocket connections have an idle timeout (default: 5 minutes). If no audio data is sent, the connection may close.

**Diagnosis:**
```bash
# Check idle timeout setting
grep WEBSOCKET_IDLE_TIMEOUT_SECS .env
```

**Solutions:**

1. **The client sends ping/pong keepalives** to prevent idle timeout. If these stop working, check for JavaScript errors.

2. **Increase idle timeout** if users need longer pauses:
   ```bash
   WEBSOCKET_IDLE_TIMEOUT_SECS=600  # 10 minutes
   ```

3. **Check network stability**: Intermittent network issues can cause drops.

### WebSocket connection fails behind proxy/firewall

**Symptoms:**
- Works on localhost but not through proxy
- Connection times out or closes immediately

**Solutions:**

1. **Caddy proxy configuration** must include WebSocket support:
   ```caddy
   handle /ws/* {
       reverse_proxy localhost:8070
   }
   ```

2. **Nginx** requires explicit WebSocket headers:
   ```nginx
   location /ws/ {
       proxy_pass http://localhost:8080;
       proxy_http_version 1.1;
       proxy_set_header Upgrade $http_upgrade;
       proxy_set_header Connection "upgrade";
       proxy_set_header Host $host;
   }
   ```

3. **Check proxy timeout settings**: Some proxies have short WebSocket timeouts.

4. **Corporate firewalls** may block WebSocket. Try HTTP mode as fallback:
   ```
   https://peekaboo.example.com/?useWebSocket=false
   ```

### Debug WebSocket messages

**Diagnosis in browser:**
```javascript
// Open DevTools → Network tab → WS filter
// Click on the WebSocket connection to see messages

// Or in console:
const ws = new WebSocket('wss://your-domain.com/ws/audio');
ws.onmessage = (e) => console.log('Received:', e.data);
ws.onerror = (e) => console.error('Error:', e);
ws.onclose = (e) => console.log('Closed:', e.code, e.reason);
```

**Server-side logging:**
```bash
# Run with debug logging
LOG_LEVEL=debug ./peekaboo

# Watch for WebSocket-related messages
journalctl --user -u peekaboo -f | grep -i websocket
```

---

## Frontend Issues

### Page loads but button doesn't respond

**Symptoms:**
- Page displays correctly
- Clicking mic button does nothing
- No errors in console

**Diagnosis:**

```javascript
// Run in browser console
document.querySelector('[data-testid="mic-button"]')?.disabled
// Should be false
```

**Solutions:**

1. **Check for JavaScript errors**: Open DevTools → Console tab

2. **Verify scripts loaded**: DevTools → Network tab → filter by JS

3. **Hard refresh**: Ctrl+Shift+R (Windows) or Cmd+Shift+R (Mac)

### Media doesn't display

**Symptoms:**
- Transcription and intent work
- Photo/audio URLs returned but don't load
- Broken image icon or silent audio

**Diagnosis:**

```bash
# Test media endpoint
curl http://localhost:8080/api/media/cat
# Get URL and test it
curl http://localhost:8080/data/media/cat/set1/photo.jpg -o test.jpg
file test.jpg
```

**Solutions:**

1. **Check media files exist**:
   ```bash
   ls -la data/media/cat/set1/
   ```

2. **If using encryption, check age key**:
   ```bash
   ls -la data/age.key
   # File should exist with 0600 permissions
   ```

3. **Re-download media**:
   ```bash
   ./scripts/source-media.sh
   ```

### CSP violations in console

**Symptoms:**
- Browser console shows "Refused to load..." errors
- Content blocked by Content-Security-Policy

**Solutions:**

1. **Check if loading external resources**: Peekaboo's CSP only allows same-origin resources. External fonts, scripts, or images will be blocked.

2. **Report unexpected CSP violations**: If core functionality is blocked, the CSP may need adjustment in `backend/api/security.go`.

---

## Server Issues

### Server won't start

**Symptoms:**
- `go run main.go` exits immediately
- Error about missing environment variables

**Solutions:**

1. **Check .env file exists**:
   ```bash
   ls -la .env
   ```

2. **Source .env before running**:
   ```bash
   export $(grep -v '^#' .env | xargs)
   go run main.go
   ```

3. **Check for port conflicts**:
   ```bash
   lsof -i :8080
   ```

### Server logs show errors

**Diagnosis:**

```bash
# View recent logs (if running as systemd service)
journalctl --user -u peekaboo -n 50

# Or run with debug logging
LOG_LEVEL=debug go run main.go
```

**Common log messages:**

| Message | Meaning |
|---------|---------|
| `database unavailable` | Can't connect to SQLite |
| `whisper server health check failed` | Can't reach whisper-server |
| `TTS disabled (PIPER_SERVER_URL not set)` | Piper TTS not configured (debug level) |
| `rate limit exceeded` | Too many requests from IP |
| `invalid API key` | LLM API key format wrong |

---

## Quick Diagnostic Commands

Run these to quickly identify issues:

```bash
# Overall health
curl http://localhost:8080/health/ready

# Test transcription (with test audio)
curl -X POST http://localhost:8080/api/transcribe \
  -F "audio=@tests/fixtures/show-me-cat.webm"

# Test intent extraction
curl -X POST http://localhost:8080/api/intent \
  -H "Content-Type: application/json" \
  -d '{"text": "show me a dog"}'

# Test media lookup
curl http://localhost:8080/api/media/dog

# Test TTS (if configured)
curl -X POST http://localhost:8080/api/speak \
  -H "Content-Type: application/json" \
  -d '{"text": "Hello"}' \
  --output /dev/null -w "%{http_code}"
# Returns 200 if TTS configured, 404 if not

# Check database
sqlite3 data/peekaboo.db "SELECT COUNT(*) FROM concepts;"

# Check whisper-server
curl http://127.0.0.1:8765/health

# Check piper-server (if configured)
curl http://localhost:5000/
```
