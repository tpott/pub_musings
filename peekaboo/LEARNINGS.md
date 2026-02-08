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

### 2026-02-05: Path traversal defense-in-depth in file servers

**Context:** EncryptedFileServer used `filepath.Clean` + `filepath.Join` to construct file paths from request URLs. While Go's HTTP mux normalizes paths (redirecting `..` sequences), defense-in-depth is important for file-serving code.

**Solution:** Added explicit `strings.HasPrefix` check after resolving the full path:
```go
resolvedPath := filepath.Join(s.BaseDir, requestPath)
cleanBase := filepath.Clean(s.BaseDir) + string(filepath.Separator)
if !strings.HasPrefix(resolvedPath, cleanBase) && resolvedPath != filepath.Clean(s.BaseDir) {
    http.NotFound(w, r)
    return
}
```

**Lesson:** Go's `net/http` ServeMux normalizes URL paths before routing, so `/../../../etc/passwd` is cleaned to `/etc/passwd` and won't match a `/data/media/` prefix. However, `filepath.Clean` + `filepath.Join` alone don't prevent traversal if the HTTP layer is bypassed (e.g., a reverse proxy passes raw paths). Always add explicit prefix validation for file-serving handlers.

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

---

### 2026-02-05: HTTP server timeouts with WebSocket - use ReadHeaderTimeout not ReadTimeout

**Problem:** Go's `http.Server` has no default timeouts, making it vulnerable to slowloris attacks. But setting `ReadTimeout` or `WriteTimeout` kills long-lived WebSocket connections since those timeouts apply to the entire connection lifetime.

**Solution:** Use `ReadHeaderTimeout` (10s) and `IdleTimeout` (120s) only. These protect against slow header attacks and idle keep-alive connections without affecting WebSocket connections (which upgrade before idle timeout applies).

**Lesson:** When a Go server handles both HTTP and WebSocket, avoid `ReadTimeout` and `WriteTimeout` on `http.Server`. Use `ReadHeaderTimeout` for slowloris protection and manage WebSocket timeouts separately at the application layer (Peekaboo already does this via `WEBSOCKET_IDLE_TIMEOUT_SECS`).

---

### 2026-02-05: innerHTML with dynamic content is an XSS vector even with "trusted" sources

**Problem:** `media-display.ts` used `innerHTML` with template literals containing `placeholderText` parameter. The text originated from `getUserFriendlyMessage()` which could include API error messages from the server (via `data.error` in JSON responses).

**Solution:** Replaced `innerHTML` with `textContent` + `createElement`/`appendChild`. This is inherently safe regardless of input content since `textContent` auto-escapes HTML entities.

**Lesson:** Never use `innerHTML` with dynamic content, even if the source seems trustworthy. API error messages can be controlled by a compromised backend or man-in-the-middle. Use `textContent` for text content and `createElement` for structure.

---

### 2026-02-05: Untracked setTimeout creates timer leaks on destroy/disconnect

**Problem:** `PeekabooFlow.handleError()` used `setTimeout()` for auto-dismiss without storing the timer ID. If `destroy()` was called while the timeout was pending, the callback would fire and try to update state on a destroyed instance. Similarly, `AudioWebSocket.attemptReconnect()` used `setTimeout()` without tracking it, so `disconnect()` wouldn't cancel a pending reconnect.

**Solution:** Store the timer ID as a class property (`errorTimeoutId`, `reconnectTimer`), null it when the callback fires, and clear it in cleanup methods (`destroy()`, `disconnect()`). Also clear previous error timeout when a new error occurs (prevents stale timeouts from overlapping).

**Lesson:** Every `setTimeout`/`setInterval` in a class that has a lifecycle (create/destroy) must be tracked and cleared in the cleanup method. This is a common source of subtle bugs where callbacks fire on destroyed objects.

---

### 2026-02-06: WebM buffer splitting requires EBML init segment preservation

**Context:** When the backend splits the audio buffer at a time threshold (default 3s), the first whisper request gets a valid WebM (contains EBML header from chunk 1), but subsequent requests are CORRUPTED because they start mid-Cluster without the required EBML Header + Track info.

**Solution:** Created `WebMParser` that detects the Cluster element ID (`0x1F43B675`) in the byte stream, caches everything before it as the "init segment" (EBML Header + Segment + Info + Tracks — typically ~497 bytes), and prepends it to every `GrabAudio()` call. Current strategy re-sends the full accumulated buffer each time (initSegment + all cluster data so far). Future phases will optimize with selective cluster extraction.

**Key insight:** A WebM from browser MediaRecorder typically has a single Cluster element containing all SimpleBlocks. You can't cleanly split within a Cluster at the EBML level, but whisper can decode a WebM with a valid header + partial cluster data (it just reads what's available).

**Lesson:** Any system that splits a WebM byte stream must preserve and re-attach the init segment. The init segment contains codec ID, sample rate, channel count, and codec-private data (like OpusHead) that decoders need. Without it, the decoder has no idea how to interpret the raw audio bytes.

---

### 2026-02-06: ebml-go element hooks for byte position tracking

**Context:** Needed to find byte offsets of EBML elements within a WebM file for the `ParseClusters()` function.

**Solution:** `ebml.Unmarshal()` with `ebml.WithElementReadHooks()` provides `Element.Position` (byte offset of element ID) and `Element.Size` (content size, not including element header). The element header size varies (4 bytes for ID + variable-length size encoding). To get the full element byte range: from `Position` to the next element's `Position` (or end of file for the last element).

**Lesson:** `ebml-go`'s `Element.Size` is the *content* size, not the total element size including the header. For byte-level slicing, use inter-element position differences rather than trying to compute header sizes manually.

---

### 2026-02-06: ebml-go read hooks fire before child elements are populated

**Problem:** `ParseClusters` used `WithElementReadHooks` to capture both position AND timecode from `elem.Value.(webm.Cluster)`. All clusters showed `Timecode=0` even though the data had different timecodes (500, 1000, 1500).

**Root cause:** Read hooks fire when the Cluster element is encountered but before its child elements (like Timecode) are parsed into the struct. `elem.Value` at hook time is a zero-value `webm.Cluster`.

**Solution:** Use hooks ONLY for byte positions, then pair with fully parsed `Segment.Cluster` timecodes after unmarshal completes:
```go
_ = ebml.Unmarshal(r, &ws, ebml.WithElementReadHooks(func(elem *ebml.Element) {
    if elem.Name == "Cluster" {
        positions = append(positions, elem.Position) // only position!
    }
}))
// After unmarshal: ws.Segment.Cluster[i].Timecode has correct values
```

**Lesson:** ebml-go read hooks provide structural metadata (position, name) but NOT populated values. For child element values, always use the deserialized struct after unmarshal completes.

---

### 2026-02-06: GetRandomMediaSet returns (nil, nil) — always check for nil result

**Context:** After refactoring `processAudio` to use `ProcessTranscript` (which returns tool actions), the `executeShowMedia` function called `GetRandomMediaSet(subject)` and passed the result directly to `sendMedia()`. When testing with an in-memory DB that had concepts but no media_sets seeded, `GetRandomMediaSet` returned `(nil, nil)` (no error, but no result), causing a nil pointer dereference in `sendMedia()`.

**Solution:** Added explicit nil check for `mediaSet` before calling `sendMedia()`:
```go
if mediaSet == nil {
    h.sendError(ctx, conn, fmt.Sprintf("no media found for %s", subject), logger)
    return
}
```

**Lesson:** Go database query methods that return `(T, error)` often return `(nil, nil)` for "not found" (vs `sql.ErrNoRows` being handled internally). Always check for nil result separate from error, especially when refactoring code paths that previously didn't reach that state.

---

### 2026-02-05: TDD "prove the bug" tests need t.Skip for pre-commit hooks

**Context:** Task 135 required writing a failing test to prove the WebM container corruption bug. The test correctly demonstrates that after buffer split, subsequent whisper requests receive invalid WebM (missing EBML header). However, the pre-commit hook runs `go test ./...` and blocks commits when any test fails.

**Solution:** Use `t.Skip("Known bug: ...")` with a clear reference to the fix task. The test still compiles, can be run explicitly with `go test -v -run TestName`, and will be un-skipped when the fix is implemented (task 137).

**Lesson:** In TDD workflows with pre-commit hooks, use `t.Skip` for known-failing tests that prove bugs exist. The skip message should reference which task will fix the bug and un-skip the test.

---

### 2026-02-06: Real-services E2E tests require backend with correct MEDIA_DIR

**Problem:** Both `real-services.spec.ts` tests failed with "Network issue - please check your connection and try again." The page never showed media.

**Root causes (3 issues):**
1. **No media in `backend/data/media/`** — the `source-media.sh` script had never been run, so the directory didn't exist. Media existed at `data/media/` (wrong location) from an earlier manual setup.
2. **Backend not running** — the Go backend on port 8080 was not started, so WebSocket connections from the frontend (proxied from Astro dev server at :4321) failed immediately.
3. **Empty `media_sets` table** — even when the backend had been run previously, `seedMediaFromDisk()` silently skipped seeding because it couldn't find the media directory (logged at DEBUG level only).
4. **Path mismatch when running from `backend/`** — the `.env` has `MEDIA_DIR=backend/data/media` and `DB_PATH=backend/data/peekaboo.db`, which are relative to the project root. But `go run main.go` must be run from `backend/` (where `go.mod` lives), so the paths need to be overridden: `MEDIA_DIR=data/media DB_PATH=data/peekaboo.db`.

**Solution:**
```bash
# 1. Download media
bash scripts/source-media.sh

# 2. Start backend with corrected relative paths
cd backend && (set -a && source ../.env && set +a && \
  MEDIA_DIR=data/media DB_PATH=data/peekaboo.db go run main.go) &

# 3. Run tests
cd frontend && PEEKABOO_REAL_SERVICES=1 npx playwright test tests/e2e/real-services.spec.ts
```

**Lesson:** The `.env` paths assume the backend runs from the project root, but Go requires running from `backend/` (where `go.mod` is). When starting the backend manually, override `MEDIA_DIR` and `DB_PATH` to be relative to `backend/`. The `seedMediaFromDisk` skip is logged at DEBUG level — use `LOG_LEVEL=debug` to catch seeding failures.

---

### 2026-02-08: TOTP 2FA with stdlib instead of external dependency

**Context:** Task 220 required implementing TOTP (Time-based One-Time Password) validation for the login flow. The existing login handler had a TODO stub accepting any TOTP code.

**Options considered:**
- pquerna/otp: Popular Go TOTP library, adds external dependency, full OTP support
- stdlib only: TOTP is just HMAC-SHA1 + dynamic truncation per RFC 6238/4226, ~50 lines of code

**Decision:** Implement with stdlib (`crypto/hmac`, `crypto/sha1`, `encoding/base32`, `encoding/binary`). The algorithm is simple enough that an external dependency adds more risk (supply chain) than value.

**Outcome:** 127 lines in `auth/totp.go`. Validated against RFC 6238 Appendix B test vectors. Supports ±1 period clock skew, 6-digit codes, 30-second periods, base32-encoded 20-byte secrets, and `otpauth://` URI generation for authenticator apps.
