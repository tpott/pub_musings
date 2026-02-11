# Help Needed

## Blocked: Production media files return 404 + local whisper-server broken

**Context:** User reported (FEEDBACK.md) that on `https://peekaboo.pottingers.us/`:
1. Photos don't render when saying "show me a cat"
2. TTS still speaks (should be suppressed by `dropTTSWithShowMedia`)
3. Real-services E2E tests don't pass

**Task:** Fix the above issues and get real-services E2E tests to pass.

### Investigation Results

#### Issue 1: Production media files return 404

**Evidence:**
- `curl -s -o /dev/null -w '%{http_code}' https://peekaboo.pottingers.us/data/media/cat/set1/photo.jpg` → **404**
- `curl -s -o /dev/null -w '%{http_code}' https://peekaboo.pottingers.us/data/media/cat/set1/photo.jpg.age` → **200**
- `curl -s https://peekaboo.pottingers.us/health` → **OK** (backend is running)
- Response body "404 page not found" is Go's default `http.NotFound`, confirming the request reaches the Go backend

**Diagnosis:** The Go backend is using the **plain `http.FileServer`** instead of the `EncryptedFileServer`. This happens when `os.Stat(ageKeyFile)` returns an error (file doesn't exist or has wrong permissions). The `.env` sets `AGE_KEY_FILE=backend/data/age.key`. If the production server doesn't have this file (or permissions are wrong), the backend falls through to the plain file server, which looks for `photo.jpg` but only finds `photo.jpg.age`.

**Fix needed (on production server):**
1. Verify `backend/data/age.key` exists (relative to `WorkingDirectory=/home/trevor/pub_musings/peekaboo`)
2. Verify permissions are 0600 or 0400 (`chmod 600 backend/data/age.key`)
3. Restart the peekaboo service: `systemctl --user restart peekaboo`
4. Verify: `curl -s -o /dev/null -w '%{http_code}' https://peekaboo.pottingers.us/data/media/cat/set1/photo.jpg` should return 200

**Alternative fix:** If encryption isn't needed, decrypt all `.age` files back to plain:
```bash
cd /home/trevor/pub_musings/peekaboo/backend/data/media
find . -name "*.age" -exec sh -c 'age -d -i ../age.key "$1" > "${1%.age}"' _ {} \;
```

#### Issue 2: TTS still speaking despite `dropTTSWithShowMedia`

**Code is correct:** `dropTTSWithShowMedia()` is called in both `parseToolActions` (Anthropic) and `parseOpenAIToolActions` (OpenAI). All backend tests pass. The TTS suppression should work.

**Possible explanations:**
- The production server may be running an older version of the code (before task 280 which added `dropTTSWithShowMedia`). Check: `git log --oneline peek1 | head -5` on the production server.
- If the LLM returns only `text_to_speech` (no `show_media`), TTS plays. This happens when media lookup fails (404), so the LLM gets an error and narrates instead.

**This is likely a symptom of Issue 1:** If media files 404, the backend sends an error to the client. On the next attempt, the LLM might respond with TTS only (no `show_media`).

#### Issue 3: Real-services E2E tests don't pass locally

**Root cause: Local whisper-server is broken.**

The whisper-server on port 9090 (`ggml-medium.bin`) returns either:
- `{"error":"FFmpeg conversion failed."}` for direct curl requests
- Empty transcripts (`text=""`) when called from the Go backend

There are 5 duplicate whisper-server processes on port 9090. The FFmpeg conversion is failing for ALL audio formats including WAV.

**Fix needed:**
1. Kill all whisper-server processes: `pkill whisper-server`
2. Restart a single instance:
   ```bash
   /home/trevor/Github/whisper.cpp/build/bin/whisper-server \
     -m /home/trevor/Github/whisper.cpp/models/ggml-medium.bin \
     -t 4 --port 9090 --convert
   ```
3. Verify: `curl -s http://127.0.0.1:9090/inference -F "file=@tests/fixtures/me-show-me-a-cat.webm" -F "response_format=verbose_json" -F "temperature=0" -F "language=en"` should return a transcript

### What passes today

- **56 E2E tests (mocked):** All pass
- **325 frontend unit tests:** All pass
- **All backend tests:** All pass
- **Code review:** `dropTTSWithShowMedia`, `EncryptedFileServer`, `sendMedia`, `showImage` — all correct

### What I cannot do from this VM

- Access or modify the production server
- Fix the local whisper-server (environment issue, not code)
- Run the real-services E2E tests (depends on working whisper)

### Recommended actions

1. **Production (urgent):** Fix the age key issue on the production server (see Issue 1 fix above)
2. **Production:** Verify the deployed code includes `dropTTSWithShowMedia` (task 280)
3. **Local:** Kill and restart whisper-server, then run real-services tests
4. **After fix:** `PEEKABOO_REAL_SERVICES=1 npx playwright test tests/e2e/real-services.spec.ts`
