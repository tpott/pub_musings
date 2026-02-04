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
