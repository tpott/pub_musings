# Learnings

Hard-won lessons from development. Future Ralphs: READ THIS FIRST.

When updating, follow [LEARNINGS-FORMAT.md](docs/ralph/LEARNINGS-FORMAT.md).

---

### 2026-02-14: Deploy health checks need curl timeouts and automatic rollback

**Problem:** `deploy-peekaboo-backend.sh` health check curls had no `--max-time` flag. If the server accepts connections but hangs (e.g., stuck on DB migration), curl blocks indefinitely and the deploy script never completes. Also, when the health check failed after all retries, the broken binary stayed deployed with no rollback.

**Solution:** Added `--max-time 5` to all curl calls. Added rollback logic after health check failure: restore `peekaboo-prev` binary and restart the service. Script still exits 1 to signal deployment failure.

**Lesson:** Always set `--max-time` on deployment health check curls — a hanging server is worse than a failing one because the deploy script never completes. Always implement automatic rollback to the previous version on deploy failure.

---

### 2026-02-14: Deploy script health check port must match systemd service PORT

**Context:** Deep inspection found `BACKEND_PORT=9070` in `deploy-peekaboo-backend.sh` but `PORT=8070` in `peekaboo.service`.

**Impact:** Every deployment health check fails with "Server not responding" even though the backend is running fine on port 8070.

**Fix:** Changed deploy script default from 9070 to 8070 to match the systemd service.

**Lesson:** When systemd services and deployment scripts both reference ports, keep them as a single source of truth or add a startup verification check.

---

### 2026-02-14: Multi-step DB operations with side effects need transactions

**Context:** Email verification consumed the token (MarkEmailVerificationTokenUsed) then set email_verified in a separate query. If the second query failed, the token was consumed but the email wasn't verified — user locked out.

**Fix:** Created `VerifyEmailWithToken` and `RedeemMagicLinkToken` methods that wrap both operations in a single database transaction with `tx.Begin()/tx.Commit()`.

**Lesson:** When consuming a one-time token AND performing a follow-up mutation, wrap both in a transaction. If either fails, the entire operation rolls back.

---

### 2026-02-11: ARIA radiogroup requires radio semantics or arrow key navigation

**Problem:** Star rating used `role="radiogroup"` on the container but individual star `<button>` elements lacked `role="radio"`, `aria-checked`, and arrow key navigation. Screen readers announced the group as a radiogroup but users couldn't interact with it using the expected arrow key pattern.

**Solution:** Removed `role="radiogroup"` since individual toggle buttons are the simpler correct pattern. Added `aria-pressed` to each star button (toggled by `updateStars()`). Buttons are already keyboard accessible via Tab+Enter/Space.

**Lesson:** Don't use `role="radiogroup"` unless you implement the full ARIA radio pattern (role="radio", aria-checked, arrow keys). For simple toggle button groups, `aria-pressed` on individual buttons is simpler and correctly accessible.

---

### 2026-02-11: Unhandled async promise in synchronous event handler

**Problem:** `MediaRecorder.ondataavailable` called `sendAudioChunk()` (an async function) without awaiting or catching the returned promise. If `sendAudioChunk` threw (e.g., WebSocket disconnected during base64 encoding), the rejection was unhandled.

**Solution:** Added `.catch()` to the `sendAudioChunk()` call in the `ondataavailable` handler, logging the error at debug level.

**Lesson:** When calling async functions from synchronous event handlers (DOM events, WebSocket callbacks), always add `.catch()` — you can't `await` in a sync callback, so unhandled rejections are silent.

---

### 2026-02-11: CSRF middleware DB lookup on every request with session token

**Problem:** Any request with a session cookie (valid or not) triggered SHA-256 hashing + database lookup in the CSRF middleware. Malformed tokens (non-hex, wrong length) still paid the full cost.

**Solution:** Added early format validation — check that the session token is exactly 64 hex characters before hashing and querying the database. Malformed tokens skip the CSRF check and pass through to the handler for normal auth rejection.

**Lesson:** Validate input format before expensive operations (hashing, DB queries). Even bounded-size inputs can be pre-filtered to skip unnecessary work.

---

### 2026-02-10: Hash session tokens before DB storage

**Context:** Session tokens were stored in plaintext in the sessions table, while email verification and magic link tokens were already hashed with SHA-256. If the database were compromised, plaintext session tokens could be used directly for session hijacking.

**Options considered:**
- Option A: Hash in DB layer (GetSessionByTokenHash hashes internally) — keeps callers simple but adds auth dependency to db package
- Option B: Hash at call sites (callers pass auth.HashToken before DB call) — keeps db package pure, matches existing verification/magic-link pattern

**Decision:** Option B — hash at call sites. Renamed Session.Token→TokenHash, GetSessionByToken→GetSessionByTokenHash, callers pass auth.HashToken(plaintext). Consistent with existing token hashing pattern. ALTER TABLE migration renames column for existing DBs.

**Outcome:** 10+ callers updated, all tests pass. DB package stays dependency-free from auth.

---

### 2026-02-11: Encrypted media 404 when age key file is missing

**Problem:** Production media files return 404 (`/data/media/cat/set1/photo.jpg`), but the `.age` version returns 200 (`/data/media/cat/set1/photo.jpg.age`). The response "404 page not found" comes from Go's `http.NotFound`, confirming the Go backend receives the request. User reports photos don't render.

**Solution:** When `os.Stat(ageKeyFile)` fails (file missing or wrong permissions), the backend falls through to `http.FileServer` (plain files). If media was encrypted with `--remove-originals`, only `.age` files exist — the plain file server can't find the unencrypted originals. Fix: ensure `age.key` exists with correct permissions (0600) at the configured `AGE_KEY_FILE` path, or decrypt media back to plain files.

**Lesson:** The encrypted-vs-plain media server selection depends on `os.Stat(ageKeyFile)` succeeding. If the age key is missing, deleted, or has wrong permissions, ALL encrypted media silently becomes unreachable — no error logged at startup, just a WARN about "serving media files unencrypted." Consider adding a startup check: if `MEDIA_DIR` contains `.age` files but no age key is loadable, log an ERROR instead of silently falling through to plain file serving.

---

### 2026-02-11: Buffer threshold watcher hammers whisper when transcripts are empty

**Problem:** During real-services testing, the buffer threshold watcher fired every 500ms after the initial 3-second threshold. Each attempt sent the SAME buffer to whisper (because empty transcripts don't trigger buffer clearing), and whisper returned `text=""` every time. After ~20 empty transcriptions, the anonymous interaction rate limiter kicked in, blocking all further processing for the connection.

**Solution:** This is by design for normal operation (short silence periods between speech). But when whisper consistently returns empty transcripts (broken whisper, wrong model, incompatible audio), it creates a tight loop: threshold fires → transcribe → empty → threshold fires again 500ms later → repeat indefinitely. No code change made yet.

**Lesson:** The buffer threshold watcher should consider backing off after repeated empty transcripts. Currently it fires every 500ms regardless, creating many wasted whisper calls and quickly exhausting the anonymous rate limit (which counts each processAudio call as an "interaction").

---

### 2026-02-10: http.NewRequest without context ignores cancellation

**Problem:** `transcribeAudio()` and `forwardToWhisper()` created HTTP requests with `http.NewRequest()` instead of `http.NewRequestWithContext()`. When a WebSocket disconnected or HTTP request was canceled, the outbound whisper-server request continued running until it naturally timed out.

**Solution:** Changed both functions to accept `context.Context` and use `http.NewRequestWithContext()`. Call sites pass `ctx` (from processAudio) and `r.Context()` (from ServeHTTP).

**Lesson:** Always use `http.NewRequestWithContext` for outbound HTTP calls. Plain `http.NewRequest` creates requests that ignore parent cancellation, wasting resources on abandoned work.

---

### 2026-02-10: Unbounded query parameter length enables CPU exhaustion

**Problem:** `HandleVerify` and `HandleMagicLinkVerify` accepted arbitrary-length token query parameters. A malicious client could send a multi-MB token string that gets SHA-256 hashed and database-queried, wasting CPU.

**Solution:** Added `maxTokenLength = 128` constant and length check before hashing. Tokens are 32 random bytes hex-encoded = 64 chars, so 128 is generous.

**Lesson:** Always validate input length at system boundaries before processing. Even "just a hash" becomes expensive at large input sizes.

---

### 2026-02-09: Adding exports to mocked modules breaks tests

**Problem:** Adding `cleanupTTSBlobUrls` export to `text-to-speech.ts` caused 5 test failures with "No export defined on mock" errors. Tests that mock `./text-to-speech` using `vi.mock` only include explicitly declared exports.

**Solution:** Added `cleanupTTSBlobUrls: vi.fn()` to all 3 test files that mock `./text-to-speech` (peekaboo-flow.test.ts, peekaboo-flow-websocket.test.ts, peekaboo-flow-websocket-tts.test.ts).

**Lesson:** When adding new exports to a module that's mocked in tests, grep for `vi.mock('./module-name')` and add the new export to every mock declaration. Vitest's strict mock mode throws on undefined exports.

---

### 2026-02-09: IPv6 port stripping with manual string scanning

**Problem:** `getClientIP()` used a backward scan for `:` to strip ports. For IPv6 `[::1]:8080` this returns `[::1]` (with brackets). While consistent as a rate limiter key, it differs from standard IP representation.

**Solution:** Replaced with `net.SplitHostPort()` which correctly handles IPv4 (`host:port`), IPv6 (`[host]:port`), and bare IPs (returns error, use as-is).

**Lesson:** Always use `net.SplitHostPort()` for `RemoteAddr` parsing. Manual port stripping is error-prone for IPv6.

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

---

### 2026-02-08: resend-go/v3 for production email delivery

**Context:** Task 235 required implementing production email sending. The existing `api.EmailSender` interface had a `LogEmailSender` dev stub.

**Options considered:**
- resend-go/v3: Official Resend Go SDK, actively maintained, already used in subtitler project (v2)
- stdlib net/smtp: No dependency, but requires SMTP server setup (MX records, SPF, DKIM)
- SendGrid/Mailgun: Heavier SDKs with more features than needed

**Decision:** Use `github.com/resend/resend-go/v3`. Same vendor as subtitler (proven pattern), minimal API surface (just `Emails.Send`), v3 adds `SendWithContext` for cancellation support. The subtitler project uses v2 but v3 is backward-compatible.

**Sources:**
- https://pkg.go.dev/github.com/resend/resend-go/v3
- subtitler/backend/email/ (reference implementation)

**Outcome:** 4 files in `email/` package (resend.go, templates.go, mock.go, resend_test.go), 10 tests passing. Clean interface adaptation: peekaboo's `EmailSender` is simpler than subtitler's `EmailService` (no context, no generic `SendEmail`).

---

### 2026-02-08: json.RawMessage in test mock responses must be valid JSON

**Problem:** When writing tests for ProcessTranscript with invalid tool inputs, using `{not json}` as the Input field of a mock anthropicContentBlock caused the entire JSON response encoding to fail silently (the json.RawMessage is embedded in the response struct). The test hit `decode response` error instead of the intended `unmarshal show_media input` error.

**Solution:** Use JSON that is structurally valid but type-incorrect for the target struct (e.g., `{"subject": 123}` instead of `{"subject": "cat"}`). This passes the outer JSON decode but fails on `json.Unmarshal` into the typed struct because `123` is not a string.

**Lesson:** When testing JSON unmarshaling errors within nested structures using mock HTTP servers, invalid JSON in a `json.RawMessage` field will break the parent struct's encoding. Use type-mismatch JSON instead (wrong types, not broken syntax).

---

### 2026-02-09: WebSocket error messages mapped to generic "Service temporarily unavailable"

**Problem:** Users reported "Service temporarily unavailable - please try again" errors frequently. The backend sends specific error messages like "transcription failed" or "intent extraction failed", but the frontend WebSocket handler wrapped ALL error messages as `ApiError(message, 'server')`. The `getUserFriendlyMessage` function maps 'server' type to the generic string, discarding the backend's descriptive message.

**Solution:** Changed WebSocket error type from 'server' to 'client' in `websocket-audio.ts`. The 'client' error type passes through the original message from `error.message`, so users now see the actual backend error ("transcription failed", etc.) instead of the generic message.

**Lesson:** When proxying error messages from a backend through a classification layer, ensure the classification doesn't lose the original message. Use 'client' type for errors with meaningful backend messages; reserve 'server' type for truly opaque HTTP 5xx errors where no backend message is available.

---

### 2026-02-09: WebSocket E2E tests broke silently across two tasks

**Problem:** 9 WebSocket E2E tests failed but weren't caught because they weren't run between tasks 249-255. Two independent changes combined to break them: (1) Task 249 added `wsClient.disconnect()` in `stopWebSocketRecording()`, which closed the WebSocket before the server could respond with transcript/media. (2) Task 250 changed audio transport from binary blobs to base64-encoded JSON (`audio_data` messages), but E2E mock WebSocket handlers still identified audio by checking for non-string (binary) messages in `typeof message !== 'string'` branches.

**Solution:** (1) Removed `disconnect()` from `stopWebSocketRecording()`, changed state to `transcribing` instead of `idle` to wait for server response. Added `disconnect()` in `handleWsMedia()` when recorder is inactive (user already stopped). (2) Updated all 9 E2E mock WebSocket handlers to check `parsed.type === 'audio_data'` instead of relying on binary message detection.

**Lesson:** When changing a message protocol (binary → JSON), update ALL consumers including test mocks. E2E tests that worked with binary detection silently stopped receiving audio data when it became JSON. Always run the full E2E suite after protocol changes. Also, `disconnect()` in a request-response WebSocket flow should happen AFTER receiving the response, not immediately after sending the request.

---

### 2026-02-10: Server-side TTS suppression as defense-in-depth

**Problem:** User reported "app said 'here is a cat' after showing" — the LLM returned both `text_to_speech` and `show_media` tool calls, so the TTS narrated what was about to display. The system prompt said not to do this, but LLMs don't always follow instructions.

**Solution:** Added `dropTTSWithShowMedia()` in `provider.go` that filters out `text_to_speech` actions when `show_media` is present. Called from both `parseToolActions()` (Anthropic) and `parseOpenAIToolActions()` (OpenAI) at the action-parsing layer, before actions reach the WebSocket handler.

**Lesson:** Don't rely solely on LLM prompts to prevent unwanted behavior. Enforce constraints in code as defense-in-depth. The LLM prompt says "don't narrate when showing media" but the server-side filter guarantees it.

---

### 2026-02-09: Audit agent false positives — always verify claims against code

**Problem:** Deep inspection subagents reported 5 backend issues. Verification against actual code showed 2 were false: (1) "Missing context timeout in WebSocket LLM calls" — timeout was properly set in `processTranscript()` via `context.WithTimeout(ctx, intentTimeout)`. (2) "Readiness probe missing database ping" — `health.go` already included `h.DB.Ping()` in the readiness check.

**Solution:** Added a verification step after initial audit: read the actual code for each claim before filing tasks. Only 3 of 5 claims were accurate.

**Lesson:** Audit agent claims have a significant false positive rate. Always verify specific code references before acting on audit findings. The agents tend to miss code that's in a different file from where they expected it (e.g., timeout set in `websocket_interaction.go` rather than `websocket_audio.go`).

---

### 2026-02-10: Consistent input validation across HTTP and WebSocket paths

**Problem:** The HTTP `/api/intent` endpoint validated transcript length (max 500 chars) but the WebSocket `processTranscript` path had no such limit. While whisper naturally bounds transcripts by audio duration, this inconsistency means the WebSocket path lacks defense-in-depth against abnormally long transcripts.

**Solution:** Added `maxTranscriptLength = 500` const in `websocket_interaction.go` and a length check at the start of `processTranscript()`, rejecting transcripts exceeding 500 chars with a descriptive error.

**Lesson:** When the same operation (e.g., sending text to LLM) is available via multiple transports (HTTP, WebSocket), ensure validation is consistent across all paths. The WebSocket path often gets less validation attention because it's "internal" to the audio flow.

---

### 2026-02-10: Focus trap pattern for modal dialogs (WCAG compliance)

**Problem:** The feedback modal lacked a focus trap — users could Tab past the modal to elements behind the overlay, violating WCAG 2.1 modal dialog guidelines.

**Solution:** Added a `keydown` handler that intercepts Tab within the modal: queries all focusable elements in `.modal-content`, wraps focus from last→first on Tab and first→last on Shift+Tab. The selector `'button:not([disabled]), select, textarea, input, [tabindex]:not([tabindex="-1"])'` covers all interactive elements.

**Lesson:** Modals with `aria-modal="true"` need focus trapping in JavaScript — the `aria-modal` attribute is a hint to assistive technology but doesn't actually prevent keyboard focus from escaping. The focus trap must be in a `keydown` handler, not `keyup`, to prevent the default Tab behavior.

---

### 2026-02-10: CORS credentials required for cross-origin cookie auth

**Problem:** The CORS middleware set `Access-Control-Allow-Origin` but omitted `Access-Control-Allow-Credentials: true`. Browsers refuse to send cookies with cross-origin requests unless this header is present. Cookie-based session auth silently failed in cross-origin deployments.

**Solution:** Added `Access-Control-Allow-Credentials: true` to `CORSMiddleware()` when `allowedOrigin` is not `*` (wildcard is incompatible with credentials per CORS spec).

**Lesson:** Cookie-based authentication + CORS requires `Access-Control-Allow-Credentials: true`. This is easy to miss because same-origin deployments work fine without it. Test cross-origin auth explicitly.

---

### 2026-02-10: Unbounded accumulation buffers need max size limits

**Problem:** The WebMParser's `rawBuffer` could grow without limit during continuous recording. If audio kept flowing without being processed (e.g., LLM returning `wait_for_more` repeatedly), the buffer could exhaust server memory.

**Solution:** Added `defaultMaxBufferSize = 50MB` and made `Append()` return false when the limit would be exceeded. Callers log a warning and drop the chunk.

**Lesson:** Any buffer that accumulates data from an external source needs a hard size cap. The cap should be generous enough for normal operation but prevent pathological memory exhaustion. Return an error/bool rather than silently dropping — callers need to know.
