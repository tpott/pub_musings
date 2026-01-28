# Learnings

Hard-won lessons from development. Future Ralphs: READ THIS FIRST.

### Format

```
### YYYY-MM-DD: Brief title

**Problem:** What happened

**Solution:** How you fixed it

**Lesson:** What future Ralphs should know
```

---

### 2026-01-28: Flex display breaks bionic reading word spacing

**Problem:** Bionic reading uses `<strong>` tags to bold first portion of words: `<strong>He</strong>llo <strong>Wor</strong>ld`. When rendered inside a `display: flex` container, each text node and `<strong>` element becomes a separate flex item, causing word spacing to collapse.

**Solution:** Changed `.modal-current-subtitle` from flex to table display:
```css
.modal-current-subtitle {
  display: table;
  width: 100%;
}
.modal-current-subtitle-inner {
  display: table-cell;
  vertical-align: middle;
}
```

**Lesson:** Flex containers treat each child (including text nodes between inline elements) as a flex item. For content with inline formatting like `<strong>`, use block/table display instead of flex to preserve natural text flow.

---

### 2026-01-28: Blob URLs trigger downloads instead of display

**Problem:** Using `window.open(blobURL)` with `text/plain` or `application/json` MIME types causes some browsers to download the file instead of displaying it in the tab.

**Solution:** Wrap content in an HTML viewer page with syntax highlighting and a copy button:
```typescript
function createViewerHTML(content: string, title: string): string {
  return `<!DOCTYPE html><html>...<pre>${escapeHtml(content)}</pre>...</html>`;
}
const html = createViewerHTML(content, 'Subtitles (SRT)');
const blobUrl = URL.createObjectURL(new Blob([html], {type: 'text/html'}));
window.open(blobUrl, '_blank');
```

**Lesson:** For "view in new tab" functionality, always serve as `text/html`. Browser behavior for blob URLs with other MIME types is inconsistent.

---

### 2026-01-22: Ralph isn't creating new tasks in TASKS.jsonl

**Problem:** Ralph will run out of explicit TASKS and will start working on implicit ones

**Solution:** When Ralph is working on a problem and notices an issue that it can work around
but should ideally address in the long term, ralph should add a new task to TASKS.jsonl. When
Ralph is getting low on the number of "todo" tasks, Ralph should spend extra time studying
current specs/, current code, current application behavior and then file new tasks to improve.

**Lesson:** Ralph should always be improving itself!

---

### 2026-01-26: hCaptcha CAPTCHA integration

**Problem:** Adding CAPTCHA to protect registration/login from bots.

**Solution:** Used hCaptcha (privacy-focused alternative to reCAPTCHA):
1. Backend `captcha` package with `Verifier` interface (allows disabled/enabled/mock modes)
2. `CAPTCHA_SITE_KEY` and `CAPTCHA_SECRET_KEY` env vars - both required to enable
3. Frontend loads hCaptcha script dynamically only when needed
4. For TOTP login flow, CAPTCHA only required on initial login, not on TOTP code entry (user already passed CAPTCHA)
5. Frontend stores `captchaPassed` state to hide widget after successful validation

**Lesson:** Optional features should gracefully degrade. Using a Verifier interface with disabled/mock implementations makes testing easy and allows development without CAPTCHA keys.

---

### 2026-01-22: Go binary not in PATH

**Problem:** `go test` failed with "command not found"

**Solution:** Use full path `/home/trevor/go/bin/go`

**Lesson:** Always check if tools are in PATH. Document full paths in AGENTS.md.

---

### 2026-01-22: Documentation without implementation

**Problem:** Task 23 created BROWSER_TESTING.md documenting `npm run test:e2e`, but:
- Playwright was never installed
- No e2e/ directory created
- No test:e2e script in package.json

The task's `done_when` was "BROWSER_TESTING.md exists" which allowed this gap.

**Solution:** Added Task 25 to actually implement the E2E test setup.

**Lesson:** Documentation tasks require verification. Run every command you document.
If `done_when` only requires docs to exist, you still must verify the docs work.

---

### 2026-01-22: Memory files were not updated

**Problem:** After 24 tasks, LEARNINGS.md was empty and no new specs were created in `specs/*.md`.
Ralph read RALPH.md but treated memory updates as optional.

**Solution:** Updated RALPH.md to make LEARNINGS.md mandatory and clarify that specs
should be created for new features.

**Lesson:** Explicit is better than implicit. If a step is important, make it a rule,
not a suggestion.

---

### 2026-01-22: Meta - How to evaluate and improve Ralph

**Context:** A human reviewed Ralph's work after 24 tasks and found systemic gaps:
empty LEARNINGS.md, unimplemented documentation, spec TODOs ignored, implementation
deviating from spec (whisper-cli vs whisper-server).

**Process used:**
1. Read RALPH.md to understand expected behavior
2. Check git history to see what was actually done
3. Analyze logs (`grep -v "=== Iteration" /tmp/ralph_logs | jq ...`) to understand decisions
4. Compare specs/docs against actual implementation
5. File tasks for every gap found
6. Update RALPH.md to prevent future mistakes but keept it short! RALPH.md and AGENTS.md get loaded
   on every subagent, so they must be token efficient with LLM inference.
7. Seed LEARNINGS.md with examples so Ralph knows what entries look like

**Lesson:** Ralph should periodically do this self-review:
- Are the specs up to date? Are TODOs getting filled in?
- Does the implementation match the spec? If not, is the spec wrong or the code?
- What did I learn that future Ralphs need to know?
- Am I just completing tasks, or am I improving the system?

Ralph isn't just a task executor - Ralph should be self-improving. File tasks to fix
gaps. Update specs when requirements change. Keep LEARNINGS.md current. The goal is
that each Ralph iteration leaves the project in a better state than it found it.

---

### 2026-01-22: Playwright webServer needs full Go path

**Problem:** When setting up Playwright's webServer config for the backend, using just `go run main.go` fails because Go isn't in PATH.

**Solution:** Use full path in playwright.config.ts:
```typescript
webServer: [
  {
    command: 'cd ../backend && /home/trevor/go/bin/go run main.go',
    ...
  }
]
```

**Lesson:** Be consistent - the same Go PATH issue applies everywhere, not just direct bash commands.

---

### 2026-01-22: Progress stuck at 30% during transcription

**Problem:** Frontend showed progress stuck at 30% during whisper transcription because:
- Backend only updated progress at discrete stages: 5% (decrypt), 10% (extract audio), 30% (start transcription)
- Whisper subprocess provides no progress callbacks
- Users saw frozen progress bar for potentially minutes on long videos

**Solution:** Added a background goroutine that simulates progress updates every 5 seconds during transcription, incrementing from 30% to 90% in 5% steps. Channel closes when whisper completes.

**Lesson:** For long-running subprocess operations without progress callbacks, simulate progress to provide user feedback. Users prefer any visible progress over a frozen UI.

---

### 2026-01-22: SRT download MIME type compatibility

**Problem:** SRT download used `Content-Type: text/srt; charset=utf-8` which is a non-standard MIME type. Some browsers might not handle this well for downloads.

**Solution:** Changed to `text/plain; charset=utf-8` which is universally supported. The `Content-Disposition: attachment` header is what triggers the download behavior, not the Content-Type.

**Lesson:** For file downloads, use standard MIME types. `Content-Disposition: attachment` controls download behavior, not the Content-Type. When in doubt, `text/plain` is the safe choice for text files.

---

### 2026-01-22: Shell script cd with relative paths

**Problem:** `scripts/lint.sh` had a bug where after `cd` to backend, the next `cd "$PROJECT_ROOT/frontend"` failed because `PROJECT_ROOT` was a relative path (`.`) which was now relative to the backend directory.

**Solution:** Make `SCRIPT_DIR` and therefore `PROJECT_ROOT` absolute paths:
```bash
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
```

**Lesson:** When a shell script uses multiple `cd` commands, always use absolute paths. Relative paths break after the first `cd`. The pattern `$(cd "$(dirname "$0")" && pwd)` converts a relative path to absolute.

---

### 2026-01-22: Self-audit to find gaps in implementation

**Problem:** After completing 36 tasks, I ran a self-audit comparing specs to implementation. Found several documented gaps including:
- 2FA recovery codes not implemented (critical security issue)
- Rate limiting not implemented (security concern)
- Several features marked "NOT IMPLEMENTED" in specs

**Solution:**
1. Created Task 37 for recovery codes and Task 38 for rate limiting
2. Implemented recovery codes immediately as it's security-critical
3. Updated specs to reflect current status

**Lesson:** Periodically run self-audits by reading all specs and comparing to implementation. The specs document what SHOULD exist; if "NOT IMPLEMENTED" appears, create a task. Critical security gaps (like 2FA recovery) should be prioritized.

---

### 2026-01-22: Recovery codes alphabet excludes ambiguous characters

**Problem:** Users copying recovery codes manually could confuse similar characters: 0/O, 1/I/L.

**Solution:** Used alphabet `ABCDEFGHJKMNPQRSTUVWXYZ23456789` which excludes:
- 0 (zero) - looks like O
- O (letter) - looks like 0
- 1 (one) - looks like I or L
- I (letter) - looks like 1 or L
- L (letter) - looks like 1 or I

**Lesson:** For user-facing codes that may be manually entered, exclude visually ambiguous characters. This reduces support burden and user frustration.

---

### 2026-01-22: Go binary name depends on directory

**Problem:** `.gitignore` had `backend/subtitler` but `go build` in the backend directory produced `backend/backend` (named after the directory, not the module).

**Solution:** Added both patterns to `.gitignore`: `backend/subtitler` and `backend/backend`.

**Lesson:** Go builds name the binary after the directory by default. When adding gitignore patterns for Go binaries, add both the expected name AND the directory name pattern.

---

### 2026-01-22: Backend returns data but frontend ignores it

**Problem:** After 38 tasks were "complete", a spec-vs-implementation audit revealed that:
- Backend's `/api/auth/totp/verify` returned `recovery_codes` in the response
- Frontend's `security.astro` didn't handle or display the codes
- Users would enable 2FA but never see their recovery codes (critical security gap!)

Similar pattern: `/api/auth/totp/recover` endpoint existed but no frontend UI to access it.

**Solution:**
1. Created Tasks 39-41 from audit findings
2. Added recovery codes display section to security.astro with copy button and confirmation
3. Added "Lost access to authenticator?" recovery flow to login.astro
4. Added rate limiting to previously unprotected TOTP endpoints

**Lesson:** When completing backend tasks, verify the frontend actually USES the data returned. API responses being "correct" doesn't mean the feature is complete. Run the full user flow end-to-end.

---

### 2026-01-22: Utility functions written but never used

**Problem:** validation.ts contains well-tested utility functions (validateEmail, validatePassword, validateTotpCode, validateVideoFile, validateSegmentTiming), but none of the .astro pages actually import or use them. Forms rely on basic HTML5 validation instead.

**Solution:** Filed Task 49 to integrate validation.ts into login.astro, register.astro, and security.astro pages.

**Lesson:** When writing utility functions, grep for their imports to verify actual usage. Tests existing doesn't mean the code is being used. Check: `grep -r "from.*validation" frontend/src/pages/`

---

### 2026-01-22: E2E tests hit rate limiting when running full suite

**Problem:** E2E auth tests passed individually but failed when running the full test suite. Each test registered a new user, and after 5 registrations in a minute (rate limit), all subsequent tests failed with "Too many requests".

**Solution:**
1. Restructured tests into groups:
   - Client-side validation tests (no API calls) - can run freely
   - Tests that share a single registered user via `test.describe.serial`
   - Removed tests that duplicate backend unit test coverage (e.g., duplicate email rejection)
2. Added comments explaining why certain tests are skipped in E2E
3. Verified the skipped behaviors are covered by backend unit tests

**Lesson:** When designing E2E tests with rate-limited APIs:
- Minimize API calls per test
- Use `test.describe.serial` to share test state (like a registered user)
- Skip tests that would trigger rate limits if they're covered by unit tests
- Document why tests are skipped to avoid future "why isn't this tested?" questions

---

### 2026-01-22: Hindi transliteration requires proper handling of consonant clusters

**Problem:** When implementing romanized Hindi to Devanagari conversion, the basic character-by-character mapping produces readable but imperfect output. For example, "namaste" becomes "नमसते" instead of the correct "नमस्ते" (with halant to form the स्त cluster).

**Solution:** Documented as a known limitation. For production-quality Hindi transliteration:
- Use GoVarnam (native Go with CGO, designed for input method editing)
- Or use Aksharamukha via Docker (120+ scripts, comprehensive)
- The basic implementation is still useful for script detection and simple cases

**Lesson:** Indic script transliteration is complex. Simple character mapping works for:
- Script DETECTION (which is Unicode range checking)
- Language DETECTION from romanized text (keyword matching)
- Basic conversions that will be read by humans (phonetically close enough)

But proper consonant cluster handling (halant/virama) requires sophisticated algorithms that understand syllable structure. File this as a future enhancement task rather than blocking on perfection.

---

### 2026-01-26: rand.Read error handling patterns in Go

**Problem:** Several places in the codebase used `rand.Read()` without checking the returned error. While crypto/rand rarely fails on modern systems, ignoring the error is bad practice and can hide issues.

**Solution:** Different handling based on context:
1. **ID generation (main.go, db.go):** Return `(string, error)` and let callers handle it - typically return HTTP 500 to user
2. **Request ID generation:** Fall back to timestamp-based ID so requests don't fail completely
3. **CSRF secret:** Store error in package-level var, return empty token on failure (fails safely)
4. **Test helpers:** Create `testGenerateID()` that panics on error (acceptable in test setup)

**Lesson:** For cryptographic random:
- Production code should handle errors explicitly
- Fallbacks are acceptable for non-critical uses (request IDs)
- Security-critical code should fail safely rather than continue with bad state
- Test code can panic since test setup failure indicates bigger problems

---

### 2026-01-26: X-Forwarded-For header trust requires explicit opt-in

**Problem:** The rate limiter blindly trusted `X-Forwarded-For` and `X-Real-IP` headers from any client. This allowed attackers to spoof their IP address by sending fake headers, completely bypassing rate limiting protection.

**Solution:** Added `TRUST_PROXY` environment variable that must be explicitly set to `true` or `1` to trust proxy headers. Default is `false` which only uses `RemoteAddr` for IP detection.

**Lesson:** Never trust client-provided headers by default:
- `X-Forwarded-For` can be set by anyone, not just proxies
- Only trust these headers when you know you're behind a trusted reverse proxy
- Make proxy trust opt-in, not opt-out
- Document clearly which environment configurations require which settings
- This applies to all similar headers: `X-Real-IP`, `X-Forwarded-Proto`, etc.

---

### 2026-01-26: Privacy leak in "My Videos" endpoint - require filter criteria

**Problem:** The `GET /api/videos` endpoint returned ALL videos in the database when called without authentication or session_id. Anonymous users could see videos from all other users including their filenames, sizes, and timestamps.

**Root cause:** The backend's `ListVideos()` function had three code paths:
1. If `userID` provided: filter by user
2. If `sessionID` provided: filter by session
3. If neither: return ALL videos (the bug!)

The frontend called `/api/videos` without passing session_id for anonymous users.

**Solution:**
1. Backend: Added check requiring either auth or session_id, returns 400 Bad Request if neither
2. Frontend: Created `session.ts` utility to generate/store session IDs in localStorage
3. Frontend: Updated `videos.astro` to check auth first, then call API with session_id if anonymous
4. Frontend: Updated `upload.astro` to pass session_id for anonymous uploads
5. Added test `TestListVideosNoFilterRejected` to verify the fix

**Lesson:** For any endpoint that can return a list of resources:
- Always require filter criteria (user ID, session ID, etc.)
- Never have a "return all" code path without explicit admin privileges
- Test the "no filter provided" case explicitly
- Consider privacy implications during code review: "What if no filter is provided?"

---

### 2026-01-26: MoltenVK GPU passthrough to QEMU VMs is blocked on macOS

**Problem:** Researched using MoltenVK + Venus to pass Vulkan GPU acceleration from macOS host to Linux guest VMs for whisper.cpp acceleration. This would eliminate the need for host-based whisper-server.

**Findings:**
1. **Good news - Upstream support exists:**
   - QEMU 9.2.0+ includes Venus patches for Vulkan passthrough
   - whisper.cpp has excellent Vulkan support (PR #2302, v1.8.3 is 12x faster)
   - virglrenderer 1.0.0+ handles Venus protocol

2. **Bad news - macOS is blocked:**
   - Venus requires DMA buffer export features that MoltenVK cannot implement on macOS
   - UTM Issue #4551 documents this as a fundamental architectural limitation
   - Custom QEMU builds with patches exist (osy's gist) but require maintaining 4+ projects
   - Even when working, performance is 75-77% of native Metal

**Solution:** Keep using the host-based whisper-server approach. It's simpler, faster, and already working.

**Lesson:** Before investing in complex GPU passthrough:
1. Check if the guest OS → host stack is actually supported (Venus needs DMA buffers)
2. Consider the maintenance burden of custom builds vs. simpler architectures
3. 25% performance penalty plus complexity usually isn't worth it vs. host-based services
4. Document research thoroughly so this doesn't get re-investigated later

---

### 2026-01-26: Go must be in PATH for automated agents

**Problem:** Claude Code (Ralph) couldn't run `go` commands because Go wasn't in PATH. The workaround was hardcoding `/home/trevor/go/bin/go` in 40+ places across scripts, docs, and config files.

**Root cause:** Claude's Bash tool runs non-interactive shells that don't source `~/.bashrc` by default. Even adding Go to `.bashrc` wasn't sufficient.

**Solution:** Ensure Go is in PATH via `~/.profile` (sourced by login shells) or by setting `BASH_ENV` to point to a file that exports PATH:
```bash
# ~/.profile
export PATH="$PATH:$HOME/go/bin"
```

**Lesson:**
1. Automated tools often run non-interactive shells that skip `.bashrc`
2. Use `~/.profile` for PATH exports needed by automated systems
3. Avoid hardcoding paths - they break portability and create maintenance burden
4. When a tool isn't found, fix the environment rather than hardcoding paths everywhere

---

### 2026-01-26: Go slog package requires initialization before use

**Problem:** After implementing structured logging with `log/slog`, backend tests started failing with nil pointer dereferences. The logging functions called `Logger.Info()`, `Logger.Error()`, etc. but `Logger` was nil because `logging.Init()` wasn't called in test setup.

**Solution:** Added an `init()` function to the logging package that sets `Logger = slog.Default()` if nil. This ensures logging always works even if `Init()` is never explicitly called:
```go
func init() {
    if Logger == nil {
        Logger = slog.Default()
    }
}
```

**Lesson:**
1. Package-level loggers should have safe defaults - never leave them nil
2. Use Go's `init()` function for defensive initialization
3. `slog.Default()` provides a reasonable default logger
4. Tests often skip initialization that production code relies on - make packages self-initializing when possible

---

### 2026-01-26: XHR uploads need manual CSRF token headers

**Problem:** Authenticated users got "Invalid or missing CSRF token" errors when uploading videos. The upload used XMLHttpRequest (XHR) for progress tracking, but XHR doesn't use the `csrfFetch` wrapper that automatically adds the CSRF header.

**Root cause:** The CSRF middleware validates all POST requests for authenticated users. The `csrfFetch` wrapper handles this for regular fetch calls, but XHR uploads bypassed it.

**Solution:** For XHR-based uploads, manually get and set the CSRF token:
```typescript
// Get token BEFORE entering Promise (can't await inside non-async callback)
const csrfToken = isAuthenticated ? await getCsrfToken() : null;

// Set header after xhr.open(), before xhr.send()
if (csrfToken) {
    xhr.setRequestHeader(CSRF_HEADER, csrfToken);
}
```

**Lesson:**
1. Any state-changing request using XHR instead of fetch needs manual CSRF handling
2. Get async values (like CSRF tokens) BEFORE entering Promise callbacks
3. The `csrfFetch` wrapper only helps with `fetch()` - XHR is on its own
4. When adding CSRF protection, audit all POST/PUT/DELETE/PATCH endpoints for XHR usage

---

### 2026-01-26: Multi-key encryption requires tracking key version per file

**Problem:** Implementing encryption key rotation required significant changes beyond just adding a new key. Files encrypted with old keys need to be decryptable while new files use the new key.

**Solution:**
1. Created `MultiKeyEncryptor` that manages multiple age keys with versioning
2. Added `key_version` column to `videos` table (migration 004)
3. Updated all encrypt calls to capture and store the key version
4. Updated all decrypt calls to use the file's stored key version
5. CLI tool (`go run ./cmd/rotate-keys`) for rotation and re-encryption

**Key architecture decisions:**
- Keys stored in `data/keys/` as `key_v1.age`, `key_v2.age`, etc.
- `current` symlink points to active key (or `currentVersion` file)
- Encryption uses current key, decryption uses file's recorded version
- Re-encryption is batched and can be run incrementally

**Lesson:**
1. Key rotation requires version tracking at the data layer, not just the crypto layer
2. All encryption/decryption calls must be audited and updated together
3. A CLI tool for operations staff is essential - rotation shouldn't require code changes
4. Zero-downtime rotation requires files to remain readable during the transition period

---

### 2026-01-26: XSS protection via escapeHtml

**Problem:** Code analysis identified potential XSS vulnerabilities through innerHTML usage with user-controlled data.

**Investigation:** Examined all innerHTML usages in frontend:
- `videos.astro` - Already uses escapeHtml for filenames and segment text ✓
- `upload.astro` - Already uses escapeHtml for segment text ✓
- `security.astro` - Missing escapeHtml for session IP addresses

**Solution:**
1. Added escapeHtml function to security.astro for session IP addresses
2. Created shared `frontend/src/utils/html.ts` utility with comprehensive tests
3. Tests verify escaping of HTML tags, scripts, event handlers, and XSS attempts

**Lesson:**
1. Even "trusted" backend data like IP addresses should be escaped - defense in depth
2. innerHTML with template literals is a common XSS vector - always audit
3. The browser's `textContent → innerHTML` trick is an effective way to escape HTML
4. Regular code audits should search for innerHTML, eval, and other dangerous patterns
5. Consider using a shared utility to avoid duplicating escapeHtml across files

---

### 2026-01-26: FFmpeg subtitle embedding - burn vs embed modes

**Problem:** User reported burning subtitles into video was very slow.

**Investigation:** The original implementation used `-vf subtitles` filter which re-encodes the entire video frame by frame. This is slow but produces subtitles that are always visible and work on any player.

**Solution:** Added two modes to the burn endpoint:
1. **burn** (default): Uses `-vf subtitles` to hardcode subtitles into video frames. Slow but universal.
2. **embed**: Uses `-c:s mov_text` to create a soft subtitle track. Fast because it just copies video/audio streams without re-encoding.

```bash
# Burn mode (slow, ~1x video duration, subtitles always visible)
ffmpeg -i input.mp4 -vf "subtitles='subs.srt'" output.mp4

# Embed mode (fast, ~seconds, subtitles can be toggled)
ffmpeg -i input.mp4 -i subs.srt -c:v copy -c:a copy -c:s mov_text output.mp4
```

**Lesson:**
1. `-c:v copy -c:a copy` avoids re-encoding - huge speed improvement
2. `-c:s mov_text` is the subtitle codec for MP4 containers
3. Soft subtitles can be toggled on/off by the player, but may not work on all players
4. Always provide both options - some users need speed, others need universal compatibility
5. Consider user feedback as task suggestions - this was exactly what the user needed

---

### 2026-01-26: FFmpeg font configuration for Indic scripts

**Problem:** User reported Hindi subtitles showed as empty boxes (□) in burned videos.

**Root cause:** FFmpeg's subtitles filter uses system fonts via fontconfig. Without fonts that support Devanagari (and other Indic scripts), characters are rendered as missing glyph boxes.

**Solution:** Added `SUBTITLE_FONT` environment variable:
1. If set, adds `FontName=<font>` to the ffmpeg subtitles filter style
2. Users must install appropriate fonts (e.g., Noto Sans Devanagari)
3. Documented font installation in INSTALL.md

```go
subtitleStyle := "FontSize=24,PrimaryColour=&HFFFFFF,..."
if subtitleFont != "" {
    subtitleStyle = fmt.Sprintf("FontName=%s,%s", subtitleFont, subtitleStyle)
}
```

**Lesson:**
1. Font support is essential for international text - test with non-Latin scripts
2. Noto fonts provide excellent Unicode coverage for many scripts
3. The problem only affects burned subtitles - SRT/VTT downloads contain correct text
4. Make font configuration optional (don't break existing users)
5. Document font requirements clearly - not everyone knows they need Indic fonts

---

### 2026-01-26: Production error messages - defense in depth

**Problem:** Many backend error responses used `err.Error()` directly, which could leak internal details like file paths, database queries, IP addresses, and API keys to users.

**Solution:** Created `backend/errmsg` package with:
1. User-friendly constants for common error types (ErrVideoNotFound, ErrTranscribeFailed, etc.)
2. `LOG_VERBOSE` env var to toggle between production (safe) and development (detailed) modes
3. Helper functions like `ForVideoNotFound(err)` that log detailed errors and return safe messages
4. Test suite that verifies production errors don't contain sensitive patterns

**Key design decisions:**
- Default to safe mode (LOG_VERBOSE=false) - you have to opt-in to dangerous behavior
- Log the detailed error before returning the safe message - debugging is still possible via logs
- User-facing validation errors (email format, password requirements) are fine to return as-is
- Only wrap errors from internal operations (database, file system, external services)

**Lesson:**
1. Never return `err.Error()` directly for internal errors - always wrap with a safe message
2. Log first, then return safe message - you need both debugging ability and security
3. Validation errors are different from internal errors - user feedback is important
4. The `SetVerbose()` function enables testing both modes without environment variable manipulation
5. Test with real-world sensitive patterns (file paths, IPs, SQL, API keys) to catch leaks

---

### 2026-01-26: Path validation - check BEFORE filepath.Clean

**Problem:** When implementing path traversal prevention, calling `filepath.Clean()` before checking for `..` segments was ineffective because `filepath.Clean("uploads/../etc/passwd")` returns `"etc/passwd"` - the traversal is normalized away before we can detect it.

**Solution:** Check for traversal patterns BEFORE calling `filepath.Clean`:
```go
// Wrong order - traversal normalized away
cleanPath := filepath.Clean(path)
if containsTraversalPatterns(cleanPath) {  // Won't catch "uploads/../etc"
    return ErrPathTraversal
}

// Correct order - check original path
if containsTraversalPatterns(path) {  // Catches "uploads/../etc"
    return ErrPathTraversal
}
cleanPath := filepath.Clean(path)
```

**Key design decisions:**
1. Check for `..` as a complete path segment, not just substring (allows `..test` as valid filename)
2. Check for null bytes which can be used in some path attacks
3. Use `isWithinDir()` for absolute path validation with proper separator handling
4. Support multiple allowed directories (uploads/, data/)
5. `SafeJoin()` validates the base directory is allowed before joining

**Lesson:**
1. `filepath.Clean()` normalizes paths but removes evidence of traversal attempts
2. Check for malicious patterns BEFORE normalization
3. `..` as a complete path segment (separated by `/`) is different from `..` as a substring in a filename
4. Always verify the final path is within allowed directories as a defense-in-depth measure

---

### 2026-01-26: Security headers - conditional HSTS

**Problem:** Need to add HSTS (Strict-Transport-Security) and Permissions-Policy headers, but HSTS should only be enabled in production (when HTTPS is actually used).

**Solution:** Use the existing `HTTPS_ONLY` environment variable to conditionally add HSTS:
```go
if auth.IsHTTPSOnly() {
    w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
}
```

**Key design decisions:**
1. HSTS conditional on HTTPS_ONLY - avoids breaking development with localhost
2. Permissions-Policy always set - disables unused features regardless of environment
3. 1-year max-age with includeSubDomains - standard production HSTS configuration
4. Test both scenarios: HTTPS_ONLY=false (no HSTS) and HTTPS_ONLY=true (HSTS present)

**Features disabled by Permissions-Policy:**
- `geolocation=()` - No location tracking needed
- `microphone=()`, `camera=()` - No audio/video recording
- `payment=()` - No payment processing
- `usb=()` - No USB device access
- `interest-cohort=()` - Opt out of Google's FLoC/Topics tracking

**Lesson:**
1. Not all security headers should be enabled unconditionally
2. HSTS + localhost = browser will refuse HTTP connections, breaking development
3. Permissions-Policy is defense-in-depth - even if XSS succeeds, malicious scripts can't access disabled features
4. Test environment variable conditions to ensure headers appear/disappear correctly

---

### 2026-01-26: Defense-in-depth: validate file paths from database before serving

**Problem:** File paths stored in the database are used directly by file-serving endpoints (`http.ServeFile`). If the database is compromised or an attacker finds a way to inject malicious paths (e.g., `/etc/passwd`), the server could serve arbitrary files.

**Solution:** Applied the `pathvalidator` package to all file-serving endpoints (video download, thumbnail, burned video). Each endpoint now validates that the file path from the database is within the allowed upload directory BEFORE calling `http.ServeFile()`.

```go
// Before serving, validate path is within allowed directories
if err := pathValidator.ValidateAbsolutePath(video.FilePath); err != nil {
    logging.ErrorContext(r.Context(), "Path validation failed", "error", err, "path", video.FilePath)
    w.WriteHeader(http.StatusForbidden)
    json.NewEncoder(w).Encode(map[string]string{"error": "Access denied"})
    return
}
http.ServeFile(w, r, videoPath)
```

**Implementation details:**
1. Created global `pathValidator` initialized with `uploadDir` at server startup
2. Added validation to three endpoints: `/api/videos/{id}/video`, `/api/videos/{id}/thumbnail`, `/api/videos/{id}/burned`
3. Returns 403 Forbidden with generic "Access denied" message (doesn't leak path details)
4. Logs the actual path and error for debugging

**Lesson:**
1. Having a security utility package is only half the battle - it must actually be USED at all vulnerable points
2. Defense-in-depth means assuming any layer could be compromised - validate even "trusted" database data
3. Error messages should not leak security-sensitive information like paths - log for operators, return generic message to users
4. Test path traversal attempts explicitly - don't assume they can't happen

---

### 2026-01-26: pathvalidator must handle relative paths from database

**Problem:** After adding path validation to file-serving endpoints, all video and thumbnail downloads returned 403 "Access denied". The pathvalidator rejected paths stored in the database because they were RELATIVE (e.g., `uploads/abc.mp4.age`) while the validator was initialized with an ABSOLUTE base directory (e.g., `/home/user/.../uploads`).

**Root cause:** When files are uploaded with a relative `UPLOAD_DIR` (like `uploads`), the encrypted file path stored in the database is also relative. The `ValidateAbsolutePath` function compared this relative path against the absolute base directory, and they never matched because `/home/.../uploads/abc.mp4.age` doesn't start with `uploads/abc.mp4.age`.

**Solution:** Updated `ValidateAbsolutePath` to convert relative paths to absolute using `filepath.Abs()` BEFORE comparing against the base directory:
```go
func (v *Validator) ValidateAbsolutePath(path string) error {
    if containsTraversalPatterns(path) {
        return ErrPathTraversal  // Check BEFORE Abs()
    }
    absPath, err := filepath.Abs(path)
    cleanPath := filepath.Clean(absPath)
    return v.validateAbsolutePath(cleanPath)
}
```

**Lesson:**
1. Path validation must handle both relative and absolute paths consistently
2. Use `filepath.Abs()` to convert relative paths before comparison
3. Check for traversal patterns BEFORE converting to absolute (to catch `../etc/passwd`)
4. Test with both relative and absolute paths in different environments
5. Database paths may be stored differently depending on how UPLOAD_DIR is configured

---

### 2026-01-27: Download buttons should use client-side generation, not server requests

**Problem:** SRT/VTT/JSON download buttons make a new HTTP request each time they're clicked. User feedback indicates this was supposed to be fixed previously but the issue persists.

**Root cause investigation:** Commit `c66a935` (2026-01-26) changed the download links to use `target="_blank"` which opens in a new tab. The commit message said "SRT/VTT/JSON links now open content in new tabs instead of downloading." This may have been a misunderstanding of the user's original intent.

The current implementation:
1. Download buttons are `<a>` tags with `href="/api/videos/{id}/subtitles.{format}"`
2. Backend sets `Content-Disposition: attachment` which triggers download
3. Each click makes a new HTTP request to the server
4. Frontend already has subtitle data loaded in `transcriptionSegments` or `editedSegments`

**Problem types:**
1. Wastes bandwidth/server resources on redundant requests
2. Downloaded data may differ from what user sees (if they made unsaved edits)
3. Poor UX - small delay while fetching data that's already in browser

**Solution:** Generate SRT/VTT/JSON files client-side from the already-loaded subtitle data:
1. Create utility functions `generateSRT()`, `generateVTT()`, `generateJSON()` in frontend
2. Use `Blob` + `URL.createObjectURL()` to create downloadable data URLs
3. Use `download` attribute on anchors with the generated URL
4. No server request needed - instant downloads
5. Always downloads what user sees (including unsaved edits if desired)

**Lesson:**
1. When debugging a "recurring" bug, check git history to understand previous "fix" attempts
2. Client-side generation is better when data is already in browser
3. Commit messages should clearly state what changed and why (the c66a935 message didn't explain the _reason_ for the change)
4. User expectations: "download" means save file instantly, not "open in new tab then download"
5. Write E2E tests that verify expected download behavior to prevent regressions

---

### 2026-01-27: NEVER use scrollIntoView() for elements in scrollable containers

**Problem:** Video scrolled out of view during playback. **THIS WAS REPORTED THREE TIMES.**

**Root cause:** The code used `scrollIntoView({ behavior: 'smooth', block: 'nearest' })` to scroll subtitle segments into view during video playback. Despite `block: 'nearest'` sounding like it would only scroll if necessary, `scrollIntoView()` scrolls **ALL ancestor scrollable containers**, including the entire page.

When:
1. The video is at the top of the page
2. The segments list is below the video
3. A segment at the bottom of the list becomes active

The browser scrolls the **entire page** to bring that segment into view, causing the video to scroll up and out of the viewport.

**Solution:** Replace `scrollIntoView()` with manual container-only scrolling:
```javascript
// NEVER use this for elements inside scrollable containers:
// el.scrollIntoView({ behavior: 'smooth', block: 'nearest' });

// Use this instead - only scrolls within the container:
function scrollIntoContainerView(container: HTMLElement, element: HTMLElement) {
    const containerRect = container.getBoundingClientRect();
    const elementRect = element.getBoundingClientRect();

    if (elementRect.top < containerRect.top) {
        container.scrollTop -= (containerRect.top - elementRect.top);
    } else if (elementRect.bottom > containerRect.bottom) {
        container.scrollTop += (elementRect.bottom - containerRect.bottom);
    }
}
```

**Lesson:**
1. **NEVER use `scrollIntoView()` for elements inside scrollable containers** - it can scroll the entire page
2. Always use manual `container.scrollTop` adjustment instead
3. Add E2E tests that verify important elements stay visible during user interactions
4. When a bug is reported multiple times, the fix wasn't thorough enough - add regression tests
5. The `block: 'nearest'` option does NOT prevent page scrolling - it just affects where the element aligns

**Files affected:**
- `frontend/src/pages/upload.astro` - Fixed `navigateToSegment()` and `updateCurrentSubtitle()`
- `frontend/src/pages/videos.astro` - Already had the correct fix
- Added E2E regression tests in `frontend/e2e/upload-flow.spec.ts`

**See also:** `specs/video-scroll-fix.md` for full analysis

---

### 2026-01-27: Whisper error messages leak internal details to clients

**Problem:** When transcription fails, the raw error message from whisper-cli/server was stored in the database and returned to clients. These messages can contain:
- Internal IP addresses (`10.0.2.2:8765`)
- File system paths (`/opt/subtitler/uploads/abc123.mp4`)
- Internal service names (`whisper-server`)
- Technical error details (`connection refused`)

This is an information disclosure vulnerability - production error messages should never reveal server internals.

**Solution:**
1. Modified `dbTranscriptionToStatus()` in `main.go` to sanitize error messages
2. When status is "error" and verbose mode is OFF (production), return generic user-friendly message
3. Raw error details only shown when `LOG_VERBOSE=true` (development)
4. Uses existing `errmsg.ErrTranscribeFailed` constant for consistency
5. Added tests `TestTranscriptionErrorMessageSanitization` and `TestTranscriptionErrorVerboseMode`

**Lesson:**
1. **Always audit what gets stored in database `message` fields** - these often get returned to clients
2. **Raw error messages should NEVER be returned in production** - use user-friendly messages
3. The existing `errmsg` package pattern should be applied consistently to all error paths
4. When adding features that store errors, trace the entire path: creation → storage → retrieval → client response

---

### 2026-01-27: Backend deploy fails with "Failed to connect to bus: No medium found"

**Problem:** Webhook-deployer service fails to restart subtitler backend. Error: `Failed to connect to bus: No medium found`. Manual `./deploy-subtitler-backend.sh` works fine.

**Investigation:**
1. The webhook-deployer runs as a systemd service
2. When systemd service tries to run `sudo systemctl restart subtitler`, it fails to connect to D-Bus
3. D-Bus socket may not be accessible from within the service context
4. Running manually as user `trevor` works because the user session has D-Bus access

**Potential solutions:**
1. **PrivateMounts=no** - Allow service to see system D-Bus socket
2. **Type=notify** with socket activation - Service auto-starts on request
3. **Environment=DBUS_SESSION_BUS_ADDRESS** - Point to system bus
4. **polkit rule** - Grant webhook service permission to restart subtitler

**Lesson:** Services running within systemd may have limited access to D-Bus. Either configure the service unit to allow D-Bus access or use alternative restart mechanisms (kill/respawn, socket activation, etc.).

---

### 2026-01-27: SRT/VTT/JSON buttons should open in new tabs, not download

**Problem:** User feedback - "SRT / VTT / JSON buttons SHOULD NOT BE DOWNLOADS. I DON'T WANT DOWNLOADS."

**Solution:** Changed from `downloadSRT()/downloadVTT()/downloadJSON()` functions to `openSRT()/openVTT()/openJSON()` which open content in new browser tabs using `window.open()` with blob URLs.

**Lesson:** Understand user intent before implementing. "View subtitles" ≠ "Download subtitles". The user wanted to preview/copy subtitle content in browser, not download files.

---

### 2026-01-27: Automated deep inspections identify technical debt

**Problem:** With 224 tasks completed, all explicit work is done. How to find improvement opportunities?

**Solution:** Used parallel exploration agents to analyze:
1. **Frontend code** - Found memory leaks (event handlers not removed on pagination), ~300 lines of duplicated code (nav bars, auth checks), missing accessibility (aria-describedby for form errors)
2. **Backend code** - Found file extension sanitization gap, missing session pagination limits, no panic recovery in WithTransaction
3. **Specs vs implementation** - deployment.md still lists chunked uploads as "Future" (already implemented), missing specs for metrics/logging/migrations
4. **Documentation** - ENV.md missing some rate limit variables, RATE_LIMITS.md doesn't mention USER_RATE_LIMIT

Filed 19 new improvement tasks (225-243) covering security, performance, code quality, and documentation.

**Lesson:**
1. When out of explicit tasks, do systematic code review with multiple perspectives
2. Memory leaks from event listeners are common - especially in pagination where list is re-rendered
3. Navigation/auth code duplication across pages indicates need for shared components
4. Specs can drift from implementation - periodic audits catch this
5. Parallel exploration agents can cover more ground faster than sequential analysis

---

### 2026-01-27: Use event delegation for paginated/dynamic lists

**Problem:** videos.astro had memory leaks from event handlers. Each pagination call:
1. Re-rendered video list HTML via `videoList.innerHTML = ...`
2. Called `attachDeleteHandlers()`, `attachRetryHandlers()`, `attachViewHandlers()`, etc.
3. These functions did `element.addEventListener()` on each button/thumbnail

While old DOM elements get garbage collected (and their handlers with them), the pattern is fragile and the repeated attach calls cluttered the code.

**Solution:** Event delegation - attach ONE listener to the parent container, handle all child interactions:

```javascript
// BEFORE: Multiple per-element handlers attached on each render
function attachDeleteHandlers() {
    videoList.querySelectorAll('.btn-delete').forEach((btn) => {
        btn.addEventListener('click', (e) => { ... });
    });
}

// AFTER: Single delegated handler, set up once
videoList.addEventListener('click', (e) => {
    const button = (e.target as HTMLElement).closest('button');
    if (button?.classList.contains('btn-delete')) {
        const videoId = button.dataset.videoId;
        deleteVideo(videoId, ...);
    }
    // Handle other button types similarly
});
```

Benefits:
1. Single listener vs. N listeners - uses less memory
2. Works for dynamically added elements without re-attaching handlers
3. Cleaner code - no "attach" functions to call after each render
4. No risk of duplicate handlers if attach called multiple times

**Lesson:**
1. For lists that get re-rendered (pagination, filtering, sorting), use event delegation
2. Use `event.target.closest('.class')` to find the relevant element (handles clicks on child elements)
3. Set up the delegated listener once at page load, not after each render
4. This pattern was already correctly used for modal segments - should have applied it everywhere
