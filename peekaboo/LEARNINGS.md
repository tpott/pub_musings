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
