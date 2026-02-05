# Learnings

Hard-won lessons from development. Future Ralphs: READ THIS FIRST.

When updating, follow [LEARNINGS-FORMAT.md](docs/ralph/LEARNINGS-FORMAT.md).

---

### 2026-02-03: CC0 media sourcing and archive.org transient failures

**Context:** Needed CC0/public domain photos and audio for 6 MVP animals

**Options considered:**
- Pixabay API: Requires API key, hotlinking blocked on CDN
- Unsplash API: Requires API key, source.unsplash.com deprecated
- Freesound.org: Requires auth for downloads
- Wikimedia Commons: Direct URLs work, many licenses
- Internet Archive: Direct URLs work, CC0 content available

**Decision:** Photos from Wikimedia Commons (resized via thumb URL), audio from Internet Archive.

**Sources:**
- Photos: https://commons.wikimedia.org (various animal files)
- Audio: https://archive.org/details/animal_201701 (cat, dog, cow, pig, chicken)
- Duck audio: https://archive.org/details/duck-sounds (separate CC0 collection)

**Lesson:** Archive.org download URLs may return transient 401 errors. Retrying usually succeeds. The script uses `-fsSL` which follows redirects correctly. Wikimedia Commons thumb URLs are reliable: `https://upload.wikimedia.org/wikipedia/commons/thumb/{path}/640px-{filename}`

---

### 2026-02-04: Playwright e2e test mocking patterns

**Context:** Setting up Playwright e2e tests for the voice-to-media flow

**Issues encountered:**
1. MediaRecorder mocking after `page.goto()` doesn't work because app JS already initialized
2. `pointerdown/pointerup` events don't trigger `mousedown/mouseup` listeners

**Solutions:**
1. Use `page.addInitScript()` to mock browser APIs BEFORE page loads
2. Use `dispatchEvent('mousedown')` and `dispatchEvent('mouseup')` to match the exact events the app listens for

**Lesson:** When testing apps that initialize on page load, mocks must be set up via `addInitScript()` before navigation. Always verify which event types the app actually listens for (pointer vs mouse vs touch).

---

### 2026-02-04: Always check git remote for module paths

**Problem:** Go module was named `github.com/trevorsmith/peekaboo` instead of the correct `github.com/tpott/pub_musings/peekaboo/backend`. The agent incorrectly guessed the GitHub username was "trevorsmith" based on the home directory owner name "trevor" rather than checking the actual git remote.

**Solution:** Fixed module path in go.mod and all imports to `github.com/tpott/pub_musings/peekaboo/backend`.

**Lesson:** ALWAYS run `git remote -v` to determine the correct repository URL before setting Go module paths. Never guess GitHub usernames from filesystem paths or usernames.

---

### 2026-02-04: Astro dev server requires proxy config for API calls

**Problem:** Clicking the microphone button in the frontend resulted in a 404 error. The frontend at `localhost:4321` made calls to `/api/transcribe`, but the Astro dev server had no proxy configuration to forward these requests to the Go backend at `localhost:8080`.

**Solution:** Added Vite proxy configuration in `astro.config.mjs`:
```javascript
vite: {
  server: {
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/data/media': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
}
```

**Lesson:** When a frontend dev server runs on a different port than the backend API server, proxy configuration is required. Relative API URLs (`/api/*`) in frontend code won't magically reach a backend on a different port. Always test the full development workflow (frontend + backend together) before marking setup as complete.

---

### 2026-02-04: Rate limiter memory exhaustion prevention

**Context:** The in-memory rate limiter stored request timestamps per IP in an unbounded map. An attacker could exhaust server memory by sending requests from many unique (spoofed) IPs.

**Solution:** Added `maxEntries` field to `RateLimiter` (default 10,000 IPs). When a new IP arrives and the map is full, the IP with the oldest last-request time is evicted. Existing IPs don't trigger eviction.

**Design choices:**
- Evict oldest by "most recent request time" not "first request time" - penalizes IPs that stopped being active
- Default 10,000 entries chosen as reasonable for typical deployment (10K IPs × ~10 timestamps × ~8 bytes ≈ 800KB)
- Eviction happens only when adding NEW IPs, not on every request

**Lesson:** Unbounded maps in rate limiters are a classic memory exhaustion vector. Always cap the size of per-client state structures.

---

### 2026-02-04: Validate private key file permissions

**Context:** The age encryption key file (`data/age.key`) is loaded via `crypto.LoadIdentityFromFile()`. If this file is world-readable (mode > 0600), anyone on the system can read the private key and decrypt all media files.

**Solution:** Added permission check in `LoadIdentityFromFile()`:
```go
mode := info.Mode().Perm()
if mode&0077 != 0 {
    return nil, ErrInsecureKeyPermissions
}
```

**Behavior:**
- Mode 0600 (owner rw): Allowed
- Mode 0400 (owner r): Allowed
- Mode 0640 (group readable): Rejected
- Mode 0644 (world readable): Rejected

**Lesson:** Private key files should always validate permissions before loading. Many tools (SSH, age CLI, GPG) do this by default. Custom loading code must implement the same check.

---

### 2026-02-04: Whisper-server requires --convert flag for webm/opus

**Context:** The frontend records audio in webm/opus format (MediaRecorder default), but transcription returned empty strings when testing with the new fixture.

**Discovery:** Testing the transcribe endpoint with `tests/fixtures/me-show-me-a-cat.webm` returned `{"text":""}`. Direct testing against whisper-server returned `{"error":"failed to read audio data"}`. Converting the webm to WAV with ffmpeg and sending that worked perfectly: "Show me a cat."

**Root cause:** The whisper-server wasn't started with the `--convert` flag. Without this flag, whisper-server cannot process webm/opus files - it only handles WAV format natively. The `--convert` flag enables automatic ffmpeg conversion for non-WAV formats.

**Solution:** Ensure whisper-server is started with `--convert` flag:
```bash
./whisper-server -m models/ggml-base.en.bin --convert -t 4 --host 127.0.0.1 --port 8765
```

**Lesson:** When browser audio (webm/opus) returns empty transcriptions but WAV works, check if whisper-server was started with `--convert`. The README already documents this flag, but it's easy to forget when starting the server manually.

---

### 2026-02-04: WebSocket library selection for Go

**Context:** Needed WebSocket library for audio streaming feature (tasks 66-69).

**Options evaluated:**

1. **gorilla/websocket** - The de facto standard for years, but archived in December 2022. While stable, no security patches or maintenance.

2. **coder/websocket** (formerly nhooyr/websocket) - Actively maintained by Coder since 2024. Idiomatic Go API with context.Context support. Used by Traefik, Vault, Cloudflare.

3. **gobwas/ws** - Zero-copy, high-performance but complex low-level API.

**Decision:** Use `github.com/coder/websocket`

**Rationale:**
- Active maintenance by Coder (funded company, not abandoned project)
- Context support throughout API matches our existing patterns
- Idiomatic Go without excessive complexity
- Proven at scale by major projects
- No breaking API changes planned

**Lesson:** When a popular library is archived (like gorilla), look for community forks. nhooyr/websocket was adopted by Coder and continues active development. Check library READMEs for "new home" announcements.

---

### 2026-02-04: WebSocket rate limiting before upgrade

**Context:** HTTP rate limiter protected `/api/transcribe` and `/api/intent` but WebSocket `/ws/audio` was unprotected. An attacker could bypass rate limiting by using WebSocket for unlimited transcription requests.

**Options considered:**
1. Rate limit per-message inside WebSocket connection (complex state, hard to enforce)
2. Limit concurrent connections per IP (doesn't prevent rapid connect/disconnect abuse)
3. Rate limit connection attempts BEFORE WebSocket upgrade (simple, blocks at HTTP layer)

**Decision:** Check rate limit in `ServeHTTP` before calling `websocket.Accept()`. Returns HTTP 429 with `Retry-After: 60` header if rate limited.

**Design:**
```go
if h.RateLimiter != nil {
    ip := getClientIP(r)
    if !h.RateLimiter.Allow(ip) {
        w.WriteHeader(http.StatusTooManyRequests)
        return
    }
}
// Then accept websocket...
```

**Benefits:**
- Reuses existing RateLimiter implementation
- Standard HTTP 429 response clients understand
- Rate limit checked before any expensive WebSocket setup
- Same 10 req/min limit as HTTP endpoints

**Lesson:** Rate limit WebSocket connections at the HTTP upgrade stage, not inside the WebSocket protocol. This way you can return standard HTTP error codes and reuse existing rate limiting infrastructure.

---

### 2026-02-04: WebSocket proxy requires separate Vite config

**Context:** WebSocket `/ws/audio` endpoint worked in backend tests but failed in production. Users saw "no chunking behavior" when microphone was turned on with WebSocket mode enabled.

**Root cause:** The Vite proxy in `astro.config.mjs` only proxied `/api/*` and `/data/media/*` routes. The WebSocket endpoint `/ws/audio` was NOT proxied, so the frontend tried to connect to the Astro dev server (port 4321) instead of the Go backend (port 8080).

**Solution:** Added WebSocket proxy configuration:
```javascript
'/ws': {
  target: 'ws://localhost:8080',
  ws: true,
}
```

And updated `docs/DEPLOY.md` Caddy config to include `/ws/*` reverse proxy.

**Lesson:** WebSocket connections need explicit proxy configuration with `ws: true` in Vite. HTTP API proxy rules don't automatically apply to WebSocket upgrades. Always test WebSocket features in the full dev environment, not just backend unit tests.

---

### 2026-02-04: Empty transcript handling in voice flow

**Context:** User reported "missing text field" error when stopping microphone recording. This happened when whisper returned empty transcript (silence or no recognizable speech).

**Root cause:** In HTTP mode, the flow was:
1. Stop recording → get audio blob
2. Transcribe → get empty string ""
3. Call /api/intent with `{text: ""}` → 400 "missing text field"

The frontend didn't validate transcript before calling the intent API.

**Solution:** Added empty transcript check in `peekaboo-flow.ts`:
```typescript
if (!transcript || transcript.trim() === '') {
  throw new ApiError('No speech detected. Please try again.', 'client');
}
```

**Lesson:** Always validate API inputs at the frontend before making requests. User-facing error messages ("No speech detected") are much clearer than raw API errors ("missing text field").

---

### 2026-02-04: WebSocket mode also needs empty transcript handling

**Context:** After implementing empty transcript handling in HTTP mode, discovered the same issue existed in WebSocket mode. When whisper returns empty text, the backend would try to extract intent and fail with a confusing "intent extraction failed" error.

**Solution:** Added same empty transcript check in `backend/api/websocket.go`:
```go
if strings.TrimSpace(transcript) == "" {
    logger.Debug("empty transcript from whisper")
    h.sendError(ctx, conn, "No speech detected. Please try again.")
    return
}
```

**Root causes of empty transcripts:**
1. Silence or background noise only
2. Audio too quiet
3. whisper-server not started with `--convert` flag (can't process webm/opus)

**Lesson:** When adding user-facing validation to one code path (HTTP), check if similar paths (WebSocket) need the same validation. Consistent error messages across transport methods improve user experience.
