# Learnings

Hard-won lessons from development. Future Ralphs: READ THIS FIRST.

### Formats

**Lessons** (bugs, surprises, workarounds):
```
### YYYY-MM-DD: Brief title

**Problem:** What happened

**Solution:** How you fixed it

**Lesson:** What future Ralphs should know
```

**Decisions** (dependency, architectural, and design choices):
```
### YYYY-MM-DD: Brief title

**Context:** What situation required a decision

**Options considered:** What alternatives were evaluated

**Decision:** What was chosen and why

**Outcome:** How it worked out (update later if needed)
```

---

## Backend

### golangci-lint v2: severity doesn't affect exit code

**Problem:** Wanted `revive` file-length-limit to produce warnings (not errors) for existing large files. Set `severity: warning` in the revive rule config expecting golangci-lint to exit 0.

**Solution:** golangci-lint treats all issues identically for exit code purposes — severity is purely cosmetic (display only in compatible output formats). Set the file-length limit high enough (2600) that current code passes, and used `//nolint:funlen` directives on route-table registration functions that are structurally large by design.

**Lesson:** Don't rely on golangci-lint severity to control CI pass/fail. Either set limits that accommodate existing code (as a ceiling for growth) or use `//nolint` directives with a reason comment for intentional exceptions. The `--issues-exit-code 0` flag exists but suppresses ALL issues, which defeats the purpose.

---

### Go file splitting: extracting handlers from main()

**Problem:** main.go grew to 6340 lines with all 50 HTTP handlers as inline closures inside main(). The test file api_test.go had a duplicate set of all handlers in its own testServer.registerHandlers() method.

**Solution:** Extract handler closures into `registerXxxHandlers(mux *http.ServeMux)` functions in separate files. Since all handlers use only package-level globals (not local variables from main), they can be moved to any file in `package main` without changes.

**Lesson:** When splitting a Go file within the same package, all unexported identifiers remain accessible across files. Test files that reference unexported functions continue to work. The api_test.go handler duplication is a separate refactoring concern — don't try to fix it in the same task. After splitting, all `go run main.go` references must change to `go run .` (compiles all files in the package).

---

### Go net/http: Content-Type must be set before WriteHeader

**Problem:** Headers set after `w.WriteHeader()` are silently ignored. 9 handlers had wrong ordering.

**Solution:** Use `httputil.RespondJSON()`/`httputil.RespondError()` which handle this correctly. After Task 349, all handlers use httputil.

**Lesson:** Always use httputil helpers for JSON responses. Never use `json.NewEncoder(w).Encode()` directly.

---

### Go slog package requires initialization before use

**Problem:** Tests crashed with nil pointer because `logging.Init()` wasn't called in test setup.

**Solution:** Added `init()` function that sets `Logger = slog.Default()` if nil.

**Lesson:** Package-level loggers should have safe defaults via Go's `init()` function. Tests often skip initialization that production code relies on.

---

### go.mod module path must match GitHub org

**Problem:** `go.mod` declared `github.com/trevor/subtitler/backend` but repo is under `github.com/tpott/`.

**Solution:** Global replace in go.mod and all .go files.

**Lesson:** Check go.mod module path matches actual GitHub repository owner.

---

### Go binary name depends on directory

**Problem:** `.gitignore` had `backend/subtitler` but `go build` produces `backend/backend` (named after directory).

**Solution:** Added both patterns to `.gitignore`.

**Lesson:** Go builds name the binary after the directory by default.

---

### Background goroutines need shutdown context checks

**Problem:** Long-running goroutines continued executing during server shutdown, leaving jobs stuck in "processing" state.

**Solution:** Added `isShuttingDown()` helper checking global `shutdownCtx`. Called at key checkpoints in transcription/burn goroutines. Progress simulation goroutines listen on `shutdownCtx.Done()`.

**Lesson:** Any background goroutine running longer than shutdown grace period (30s) needs periodic shutdown checks.

---

### rand.Read error handling patterns in Go

**Problem:** Several places used `rand.Read()` without checking errors.

**Solution:** Different patterns by context: return error for ID generation, fall back to timestamp for request IDs, fail safely for CSRF, panic in tests.

**Lesson:** Cryptographic random errors should be handled explicitly. Security-critical code should fail safely rather than continue with bad state.

---

### Path validation: check BEFORE filepath.Clean

**Problem:** `filepath.Clean("uploads/../etc/passwd")` returns `"etc/passwd"` -- traversal normalized away before detection.

**Solution:** Check for `..` traversal patterns BEFORE calling `filepath.Clean()`, then normalize.

**Lesson:** `filepath.Clean()` removes evidence of traversal attempts. Always validate raw path first. Also handle relative paths from database with `filepath.Abs()`.

---

### FFmpeg subtitle embedding: burn vs embed modes

**Problem:** Burning subtitles was very slow (re-encodes entire video).

**Solution:** Added two modes: `burn` (hardcode with `-vf subtitles`, slow but universal) and `embed` (soft subtitles with `-c:v copy -c:a copy -c:s mov_text`, fast).

**Lesson:** `-c:v copy -c:a copy` avoids re-encoding. Always provide both options for user choice.

---

### FFmpeg font configuration for Indic scripts

**Problem:** Hindi subtitles showed as empty boxes in burned videos.

**Solution:** Added `SUBTITLE_FONT` env var for configuring fontconfig. Users must install appropriate fonts (e.g., Noto Sans Devanagari).

**Lesson:** Font support is essential for international text. Make font config optional; document requirements clearly.

---

### FFmpeg filter paths need escaping

**Problem:** Special characters in file paths (quotes, colons, brackets) break ffmpeg filter syntax.

**Solution:** Created `escapeFFmpegFilterPath()` to escape special chars. Also sanitize font name from `SUBTITLE_FONT` env var.

**Lesson:** FFmpeg filter strings have their own escaping rules separate from shell escaping.

---

### Whisper error messages leak internal details

**Problem:** Raw error messages from whisper containing IPs, paths, and service names were returned to clients.

**Solution:** Sanitize in `dbTranscriptionToStatus()`. Production mode returns generic message; verbose mode shows details.

**Lesson:** Always audit what gets stored in database `message` fields -- these often get returned to clients. Use `errmsg` package pattern consistently.

---

### Production error messages: defense in depth

**Problem:** Many backend error responses used `err.Error()` directly, leaking file paths, database queries, IPs.

**Solution:** Created `backend/errmsg` package with user-friendly constants. `LOG_VERBOSE` env var toggles modes. Log detailed error, return safe message.

**Lesson:** Never return `err.Error()` for internal errors. Validation errors (email format, password requirements) are fine to return as-is.

---

### Hindi transliteration: consonant clusters are complex

**Problem:** Simple character-by-character mapping produces readable but imperfect Devanagari output (missing halant clusters).

**Solution:** Documented as known limitation. For production quality, use GoVarnam or Aksharamukha.

**Lesson:** Indic script transliteration is complex. Simple mapping works for detection and basic cases. Don't block on perfection -- file as future enhancement.

---

### Progress stuck at 30% during transcription

**Problem:** Whisper subprocess provides no progress callbacks. Users saw frozen progress bar.

**Solution:** Background goroutine simulates progress every 5 seconds from 30% to 90%.

**Lesson:** For long-running operations without progress callbacks, simulate progress for better UX.

---

## Frontend

### CRITICAL: Playback speeds must be 0.8x, 0.9x, 1.0x ONLY

**Problem:** Speeds were expanded to 6 speeds (0.5x-2x) multiple times. Owner keeps reverting it.

**Solution:** Restricted to [0.8, 0.9, 1.0]. Added regression tests that assert forbidden values. Updated spec with prominent warning.

**Lesson:** DO NOT expand playback speeds without explicit owner approval. When specs and user feedback conflict, user feedback wins. If a "fix" keeps getting reverted, the spec is wrong, not the code.

---

### Browsers reset playbackRate when video source changes

**Problem:** Setting `video.playbackRate` before the video finishes loading doesn't persist. When a new `src` is assigned, browsers reset `playbackRate` to 1.0 after the media loads.

**Solution:** Set `playbackRate` immediately for responsiveness, then add a `loadeddata` event listener (with `{ once: true }`) to re-apply the saved speed after the browser resets it.

**Lesson:** Always re-apply `playbackRate` on `loadeddata` when setting `video.src`. The initial assignment before load is a no-op.

---

### Feedback modal must preserve parent modal overflow state

**Problem:** FeedbackButton's `closeModal()` set `document.body.style.overflow = ''`, which removed the `overflow: hidden` that a parent video modal had set. This broke scrolling behavior when feedback was opened over the video player.

**Solution:** Save `document.body.style.overflow` before opening the feedback modal, restore it on close instead of always clearing to empty string.

**Lesson:** Any modal that modifies `document.body.style.overflow` should save/restore the previous value, not assume it was empty. Multiple overlapping modals each need their own overflow state management.

---

### NEVER use scrollIntoView() for elements in scrollable containers

**Problem:** Video scrolled out of view during playback. **Reported THREE TIMES.** `scrollIntoView()` scrolls ALL ancestor containers including the page.

**Solution:** Use manual `container.scrollTop` adjustment instead.

**Lesson:** `scrollIntoView()` scrolls all ancestors. `block: 'nearest'` does NOT prevent page scrolling. For scrollable containers, always use manual scrollTop calculation.

---

### Flex display breaks bionic reading word spacing

**Problem:** `<strong>` tags inside `display: flex` containers cause word spacing to collapse.

**Solution:** Changed from flex to table display for subtitle containers.

**Lesson:** Flex treats each child (including text nodes) as a flex item. Use block/table display for content with inline formatting.

---

### Blob URLs trigger downloads instead of display

**Problem:** `window.open(blobURL)` with `text/plain` causes browsers to download instead of display.

**Solution:** Wrap in HTML viewer page with `text/html` MIME type and copy button.

**Lesson:** For "view in new tab", always serve as `text/html`. Browser behavior for blob URLs with other MIME types is inconsistent.

---

### Client-side subtitle generation instead of server requests

**Problem:** Download buttons made HTTP requests for data already loaded in browser. Wasted bandwidth; downloads didn't include unsaved edits.

**Solution:** Generate SRT/VTT/JSON client-side with `Blob` + `URL.createObjectURL()`. Instant downloads, includes edits.

**Lesson:** When data is already in browser, generate files client-side. Write E2E tests verifying no network requests.

---

### Use event delegation for paginated/dynamic lists

**Problem:** Memory leaks from event handlers re-attached on each pagination render.

**Solution:** Single delegated listener on parent container using `event.target.closest('.class')`.

**Lesson:** For re-rendered lists, use event delegation. Set up once at page load, works for dynamically added elements.

---

### XHR uploads need manual CSRF token headers

**Problem:** XHR uploads bypassed `csrfFetch` wrapper, getting "Invalid or missing CSRF token" errors.

**Solution:** Manually get CSRF token before entering Promise, set via `xhr.setRequestHeader()`.

**Lesson:** Any state-changing XHR request needs manual CSRF handling. Get async values BEFORE entering callbacks.

---

### HTML hardcoded values must match TypeScript constants

**Problem:** Speed buttons had `data-speed="0.8"` in HTML but PLAYBACK_SPEEDS array had different values.

**Solution:** Ensure HTML and TypeScript match. Generate HTML from same source of truth when possible.

**Lesson:** Duplicated constants between HTML and TypeScript will drift.

---

### localStorage error handling: console.error is correct for utilities

**Problem:** Deep inspection flagged console.error as "silent failure" needing user feedback.

**Solution:** No change needed. Internal utilities should degrade gracefully with console.error, returning defaults.

**Lesson:** Not every console.error needs user-facing feedback. Reserve user-facing errors for operations the user explicitly initiated.

---

## Security

### X-Forwarded-For header trust requires explicit opt-in

**Problem:** Rate limiter blindly trusted `X-Forwarded-For`, allowing IP spoofing to bypass rate limiting.

**Solution:** Added `TRUST_PROXY` env var (default false). Only trusts proxy headers when explicitly enabled.

**Lesson:** Never trust client-provided headers by default. Make proxy trust opt-in.

---

### Privacy leak in "My Videos" endpoint

**Problem:** `GET /api/videos` without auth or session_id returned ALL videos from ALL users.

**Solution:** Backend requires either auth or session_id, returns 400 if neither. Frontend passes session_id for anonymous users.

**Lesson:** List endpoints must always require filter criteria. Never have a "return all" code path without admin privileges.

---

### XSS protection via escapeHtml

**Problem:** innerHTML usage with user-controlled data (session IPs, segment text).

**Solution:** Created shared `frontend/src/utils/html.ts` with comprehensive tests. Applied escapeHtml to all innerHTML uses.

**Lesson:** Even "trusted" backend data should be escaped (defense in depth). Audit innerHTML, eval, and template literals regularly.

---

### Security headers: conditional HSTS

**Problem:** HSTS breaks localhost development.

**Solution:** HSTS conditional on `HTTPS_ONLY` env var. Permissions-Policy always set.

**Lesson:** Not all security headers should be enabled unconditionally. Test environment variable conditions.

---

### Defense-in-depth: validate file paths from database before serving

**Problem:** Database paths used directly by `http.ServeFile` could serve arbitrary files if DB is compromised.

**Solution:** Validate all paths are within allowed directories before serving. Returns generic "Access denied" message.

**Lesson:** Validate even "trusted" database data. Having a security package isn't enough -- it must be used at all vulnerable points.

---

### Multi-key encryption requires version tracking per file

**Problem:** Key rotation requires files encrypted with old keys to remain decryptable.

**Solution:** `MultiKeyEncryptor` with versioning. `key_version` column in database. CLI tool for rotation.

**Lesson:** Key rotation requires version tracking at the data layer. Zero-downtime rotation requires files to remain readable during transition.

---

### hCaptcha integration pattern

**Problem:** Adding CAPTCHA without breaking development/testing.

**Solution:** `Verifier` interface with disabled/enabled/mock modes. Loads script dynamically. CAPTCHA only on initial login, not TOTP step.

**Lesson:** Optional features should gracefully degrade. Interface with mock implementations makes testing easy.

---

## Infrastructure & E2E Testing

### Go binary not in PATH for automated agents

**Problem:** Go wasn't in PATH for Claude Code's non-interactive shells.

**Solution:** Use `~/.profile` for PATH exports. Avoid hardcoding paths.

**Lesson:** Automated tools run non-interactive shells that skip `.bashrc`. Use `~/.profile` for PATH exports.

---

### Playwright webServer needs full Go path

**Problem:** `go run main.go` fails in Playwright's webServer config because Go isn't in PATH.

**Solution:** Use full path `/home/trevor/go/bin/go run .` in playwright.config.ts. Note: must use `go run .` (not `go run main.go`) since the backend is split into multiple files.

**Lesson:** Playwright webServer commands run in stripped-down environment. Use absolute paths. With multi-file packages, always use `go run .` not `go run main.go`.

---

### E2E route mocks must include wildcard for query params

**Problem:** `page.route('**/api/upload/init')` doesn't match URLs with query parameters.

**Solution:** Always use `**/api/endpoint*` (trailing `*`) in route patterns.

**Lesson:** When writing E2E tests with mocked routes, add trailing `*` if frontend might append query parameters.

---

### Zod schemas: .optional() vs .nullable() for Go backends

**Problem:** Go nil `*string` serializes as JSON `null`, but Zod `.optional()` rejects `null`.

**Solution:** Use `.optional().nullable()` for Go pointer types.

**Lesson:** Go JSON encoding sends `null` for nil pointers, not undefined. Zod schemas need `.nullable()`.

---

### E2E cookie consent must use page.addInitScript()

**Problem:** Setting localStorage after `page.goto()` was too late -- page JS had already read the value.

**Solution:** Use `page.addInitScript()` BEFORE `page.goto()`.

**Lesson:** For localStorage values that affect page initialization, use `page.addInitScript()` which runs before page scripts.

---

### E2E tests hit rate limiting when running full suite

**Problem:** Each test registered a new user; after 5 registrations/minute, subsequent tests failed.

**Solution:** Share registered user via `test.describe.serial`. Skip tests covered by backend unit tests.

**Lesson:** Minimize API calls per test. Use serial describe blocks to share state. Document why tests are skipped.

---

### Pre-commit hook path must account for monorepo structure

**Problem:** `$(git rev-parse --show-toplevel)/scripts` resolved to wrong path because project is a subdirectory.

**Solution:** Changed to `$(git rev-parse --show-toplevel)/subtitler/scripts`.

**Lesson:** In monorepos, `git rev-parse --show-toplevel` returns the parent repo root.

---

### Videos page inline vs modal download buttons use different patterns

**Problem:** Inline buttons trigger file download; modal buttons open viewer in new tab. Tests used wrong event type.

**Solution:** Use `page.waitForEvent('download')` for inline, `page.waitForEvent('popup')` for modal.

**Lesson:** Check which JS function each button calls before writing E2E assertions.

---

### Backend deploy fails with "Failed to connect to bus: No medium found"

**Problem:** Webhook-deployer systemd service can't run `systemctl restart` due to D-Bus access restrictions.

**Solution:** Pending. Options: PrivateMounts=no, socket activation, polkit rule, or alternative restart mechanism.

**Lesson:** Services within systemd may have limited D-Bus access.

---

### Pre-commit hook E2E tests require running servers

**Problem:** The pre-commit hook runs E2E tests via `test-e2e.sh`, which require both the frontend dev server (port 4321) and backend server (port 8080) to be running. When Ralph commits without servers running, all E2E tests fail with `ERR_CONNECTION_REFUSED`.

**Solution:** Use `verify-all.sh` (lint + unit tests) for Ralph's verification. E2E tests should only run when servers are available. The pre-commit hook blocks commits in environments without running servers.

**Lesson:** When committing in an environment without dev servers, the pre-commit hook will fail on E2E tests even if unit tests pass. This is an infrastructure constraint, not a code issue.

---

## Process

### Python scripts should defer third-party imports for CLI usability

**Problem:** `evaluate.py` crashed on `--help` due to uninstalled dependencies imported at module level.

**Solution:** Moved imports into function called only when needed.

**Lesson:** For Python CLI tools with optional deps, defer imports so basic subcommands work without installing anything.

---

### Documentation without implementation

**Problem:** Task created BROWSER_TESTING.md documenting E2E tests, but Playwright was never installed and no tests existed.

**Solution:** Added separate task to actually implement E2E setup.

**Lesson:** Documentation tasks require verification. Run every command you document.

---

### Memory files must be kept updated

**Problem:** After 24 tasks, LEARNINGS.md was empty, no specs created.

**Solution:** Made LEARNINGS.md updates mandatory in RALPH.md.

**Lesson:** Explicit is better than implicit. If a step is important, make it a rule.

---

### Backend returns data but frontend ignores it

**Problem:** Backend returned recovery codes in TOTP verify response but frontend didn't display them.

**Solution:** Added display section and recovery flow to frontend.

**Lesson:** When completing backend tasks, verify the frontend actually USES the returned data. Run full user flow end-to-end.

---

### Utility functions written but never used

**Problem:** validation.ts had tested utility functions that no page imported.

**Solution:** Filed task to integrate. Grep for imports to verify usage.

**Lesson:** Tests existing doesn't mean code is being used. Check: `grep -r "from.*validation" frontend/src/pages/`

---

### Self-audit process for finding gaps

**Problem:** After many tasks, specs drift from implementation and gaps accumulate.

**Solution:** Periodically review: Are specs up to date? Does implementation match? What's missing?

**Lesson:** Ralph isn't just a task executor -- Ralph should be self-improving. File tasks to fix gaps.

---

### Deep inspections identify technical debt

**Problem:** All explicit tasks done. How to find improvement opportunities?

**Solution:** Use parallel exploration agents to analyze frontend (memory leaks, duplication, accessibility), backend (security, performance), specs vs implementation, and documentation gaps.

**Lesson:** When out of tasks, do systematic code review. Memory leaks from event listeners are common in paginated lists. Specs drift from implementation -- audit periodically.

---

### Verify test coverage before filing tasks

**Problem:** Exploration agent reported missing tests that actually existed.

**Solution:** Verify by checking for `*_test.go` or `.test.ts` files before filing.

**Lesson:** When an agent reports missing tests, verify manually first.

---

### Ralph should create new tasks proactively

**Problem:** Ralph runs out of explicit tasks and works on implicit ones without tracking.

**Solution:** When noticing issues during work, add new tasks to TASKS.jsonl. When low on tasks, study specs, code, and app behavior to find improvements.

**Lesson:** Ralph should always be improving the project, not just executing tasks.

---

### Shell script cd with relative paths

**Problem:** After `cd` to backend, `cd "$PROJECT_ROOT/frontend"` failed because PROJECT_ROOT was relative.

**Solution:** Make paths absolute: `SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"`.

**Lesson:** When using multiple `cd` commands, always use absolute paths.

---

### Recovery codes alphabet excludes ambiguous characters

**Problem:** Users could confuse similar characters: 0/O, 1/I/L.

**Solution:** Used alphabet `ABCDEFGHJKMNPQRSTUVWXYZ23456789`.

**Lesson:** For user-facing codes, exclude visually ambiguous characters.

---

### SRT download MIME type compatibility

**Problem:** `text/srt` is a non-standard MIME type.

**Solution:** Use `text/plain; charset=utf-8`. `Content-Disposition: attachment` controls download behavior.

**Lesson:** For file downloads, use standard MIME types.

---

### Splitting large Astro pages: state objects + callbacks pattern

**Problem:** upload.astro was 3844 lines with tightly-coupled script code. Functions referenced dozens of DOM elements and shared mutable state, making extraction difficult.

**Solution:** Used "state objects + callbacks" pattern: each extracted module defines interfaces for mutable state (`*State`), DOM element refs (`*Elements`), and page interaction callbacks (`*Callbacks`). Factory functions (`create*State()`) initialize state. The page creates state objects and passes them to module functions. CSS extracted to a separate `.css` file imported via Astro frontmatter (becomes global but only loads on that page).

**Lesson:** For large Astro pages, extract logic into utility `.ts` files with explicit state/element/callback interfaces. This decouples modules from DOM without needing a framework. Astro `<style>` is scoped; imported `.css` is global — fine for page-level components where class names are specific enough. The script section is usually the biggest win; CSS extraction is secondary.

---

### Frontend file size linting: shell script beats ESLint

**Problem:** Needed file size enforcement for Astro/TS files (matching backend's golangci-lint file-length-limit). Three options evaluated: (1) ESLint with eslint-plugin-astro + `max-lines` rule, (2) Biome linter, (3) simple shell script with `wc -l`.

**Solution:** Shell script (`scripts/lint-frontend-filesize.sh`) with two thresholds: error at 4000 lines, warning at 1000 lines. Test files excluded. ESLint was rejected because it requires 4+ new dev dependencies (eslint, eslint-plugin-astro, @typescript-eslint/parser, typescript-eslint) for a single rule (`max-lines`). Biome was rejected because it doesn't support .astro files.

**Lesson:** Follow the dependency policy: don't add heavyweight tools for functionality achievable with a small script. The backend set golangci-lint limits "above current largest" to cap growth — same strategy works for frontend. `wc -l` integrated into `lint.sh` gives the same effect as ESLint's `max-lines` with zero dependencies.

---

### Claude Code session log structure

**Problem:** Needed to parse Claude Code logs for ralph optimizer.

**Solution:** JSONL format with `type` fields (system, assistant, user). Tool calls in `message.content` with `type: "tool_use"`. Token usage split across `input_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`.

**Lesson:** Sum all three token fields for total input tokens. Agent logs in `$session_id/$agent_id.jsonl` subdirectories.

---

## Decisions

### 2025-01-01: SQLite over PostgreSQL

**Context:** Needed a database for users, sessions, videos, and transcriptions. Single-server deployment, no horizontal scaling requirement.

**Options considered:** (1) PostgreSQL — full-featured RDBMS, overkill for single server; (2) SQLite via `go-sqlite3` — zero-config, file-based, embedded; (3) `modernc.org/sqlite` — pure Go SQLite, no CGo, but slower.

**Decision:** SQLite via `github.com/mattn/go-sqlite3`. Single-server app doesn't need network database. CGo driver is battle-tested and fast. Connection pooling and WAL mode handle concurrent reads well.

**Outcome:** Works well. 9 versioned migrations, no operational overhead. Occasional "database locked" errors addressed with connection pooling and maintenance scheduler. Would only reconsider if multi-server deployment became necessary.

---

### 2025-01-01: Astro over Next.js for frontend

**Context:** Needed a frontend framework for a multi-page web app with server-proxied API.

**Options considered:** (1) Next.js — SSR-focused, heavier, React ecosystem; (2) Astro — static-site generator, component islands, file-based routing, built-in dev proxy; (3) Plain HTML — no component model or dev tooling.

**Decision:** Astro with static output. Pages are server-rendered at build time, interactive behavior via vanilla TypeScript `<script>` tags. No React/Vue/Svelte runtime needed.

**Outcome:** Good fit. Pages are fast (no JS framework runtime). The trade-off is that complex interactivity (like the subtitle editor) requires manual DOM manipulation, which led to large page files. Addressed via the state objects + callbacks extraction pattern (see Splitting large Astro pages entry above).

---

### 2025-01-01: age over NaCl/GPG for file encryption

**Context:** Needed file-at-rest encryption for uploaded media files.

**Options considered:** (1) `filippo.io/age` — modern, simple API, X25519 + ChaCha20-Poly1305; (2) NaCl/libsodium — lower-level, more code; (3) GPG — heavier, worse Go API.

**Decision:** age library. Authored by Filippo Valsorda (Go cryptography maintainer), simple streaming API, well-suited for file encryption.

**Outcome:** Works well. Added key rotation with `MultiKeyEncryptor` and `key_version` tracking per file. CLI tool for re-encryption during rotation.

---

### 2026-01-29: Shell script over ESLint for frontend file size linting

**Context:** Needed file size enforcement for Astro/TS files (matching backend's golangci-lint). Three options evaluated.

**Options considered:** (1) ESLint + `eslint-plugin-astro` + `max-lines` rule — 4+ new dev dependencies for 1 rule; (2) Biome — no .astro file support; (3) Shell script with `wc -l` — zero dependencies.

**Decision:** Shell script (`scripts/lint-frontend-filesize.sh`) with error at 4000 lines, warning at 1000 lines. Integrated into `scripts/lint.sh`.

**Outcome:** Clean, maintainable, zero-dependency. Same "ceiling above current largest" strategy as the backend's golangci-lint config.

---

### 2026-01-29: Python over Bash for fetch-feedback script

**Context:** `fetch-feedback.sh` used curl and jq. Needed to resolve secrets from multiple sources (env vars, `.env` file, `secrets.enc.yaml` via sops).

**Options considered:** (1) Bash with curl/jq — fragile string handling, hard to test; (2) Python with stdlib only — structured, testable, no new dependencies.

**Decision:** Python (`scripts/fetch-feedback.py`) using only `urllib`, `json`, `subprocess`. No pip dependencies.

**Outcome:** More robust, testable (5 tests in `test_ralph.py`), easier to maintain. Secrets resolution is clean with fallback chain.

---

### 2025-01-01: Resend over SendGrid/SMTP for transactional email

**Context:** Needed email delivery for verification, password reset, and magic links.

**Options considered:** (1) Resend — simple API, official Go SDK; (2) SendGrid — heavier SDK; (3) Mailgun — similar to Resend; (4) `net/smtp` with own SMTP server — operational overhead.

**Decision:** Resend. Clean API, official Go SDK (`resend-go/v2`), free tier sufficient for current usage. Disabled when `EMAIL_ENABLED=false`.

**Outcome:** Works well. Conditional enablement means dev/test environments don't need email config.

---

### 2026-01-29: Shell script for doc sync linting

**Context:** 5 documentation files (deps.md, ENV.md, API.md, RATE_LIMITS.md, ERROR_CODES.md) can drift out of sync with code. Need automated detection.

**Options considered:** (1) Go program using `go/parser` AST — precise but heavy, new dependency; (2) ESLint custom rules — wrong language for Go backend; (3) Shell script with grep — zero deps, catches 80% of drift, matches project convention from `lint-frontend-filesize.sh`.

**Decision:** Shell script (`scripts/lint-doc-sync.sh`). Five checks: deps vs go.mod/package.json, env vars vs os.Getenv calls, routes vs HandleFunc registrations, rate limits vs config. Errors for critical drift (deps, env vars), warnings for informational (routes). Integrated into lint.sh.

**Outcome:** Immediately found missing Chunked Upload category in RATE_LIMITS.md and stale Configuration section. Validates the approach — even simple grep-based checks catch real drift.
