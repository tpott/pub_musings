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

### Transcription returns empty text

**Symptoms:**
- API returns `{"text": ""}` even with clear audio
- No errors in logs

**Solutions:**

1. **Check audio format**: Whisper works best with WAV files. The `--convert` flag enables ffmpeg conversion for other formats.

2. **Check audio quality**: Very short recordings (< 1 second) or very quiet audio may produce empty transcripts.

3. **Try a larger model**: The tiny model may miss quiet or unclear speech:
   ```bash
   ./models/download-ggml-model.sh base.en
   ```

4. **Test with known-good audio**:
   ```bash
   curl -X POST http://localhost:8080/api/transcribe \
     -F "audio=@tests/fixtures/show-me-cat.webm"
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
