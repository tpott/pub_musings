# Feedback Fetch Pipeline

## Overview

A script and secrets management system that allows the development VM (claude4) to pull user feedback from production (`subtitler.pottingers.us`) and write it to `FEEDBACK.md`, creating a feedback loop for Ralph.

## Architecture

```
Production Server                 Dev VM (claude4)
┌─────────────────────┐          ┌─────────────────────────────┐
│ subtitler.pottingers │  HTTPS  │ scripts/fetch-feedback.sh   │
│ .us                  │◄────────│                             │
│                      │         │ reads secrets.enc.yaml      │
│ GET /api/admin/      │         │ calls /api/admin/feedback    │
│   feedback           │─────────│ writes FEEDBACK.md          │
└─────────────────────┘         └─────────────────────────────┘
```

## Components

### 1. Encrypted Secrets File

**File:** `secrets.enc.yaml` (git-tracked, encrypted with age)

**Plaintext structure:**
```yaml
prod_host: https://subtitler.pottingers.us
api_session_id: <session cookie value>
```

**Encryption:** Use the project's existing age encryption (see `specs/encryption.md`). The dev VM has the decryption key.

**Alternative:** If age integration is complex for a shell script, use [sops](https://github.com/getsops/sops) with age backend, or a simpler approach: a `.env.local` file (gitignored) that the user creates manually:

```bash
# .env.feedback (gitignored)
PROD_HOST=https://subtitler.pottingers.us
API_SESSION_ID=<session cookie value>
```

### 2. Fetch Script

**File:** `scripts/fetch-feedback.sh`

**Behavior:**
1. Load secrets from `.env.feedback` (or decrypt `secrets.enc.yaml`)
2. Call `GET /api/admin/feedback?status=new&limit=50` with auth header
3. Track last-fetched timestamp in `.feedback-cursor` (gitignored) to avoid repeats
4. Filter out feedback already seen (by ID or created_at > cursor)
5. Format new feedback items as markdown
6. Write to `FEEDBACK.md` (append if exists, create if not)
7. Exit 0 if new feedback found, exit 1 if none

**Auth:** `curl -H "Authorization: Bearer $API_SESSION_ID" "$PROD_HOST/api/admin/feedback?status=new"`

**Output format for FEEDBACK.md:**
```markdown
* [bug] Rating: 4/5 - "The upload button doesn't work on mobile" (2025-01-28, page: /upload)
* [feature] - "Add dark mode to the editor" (2025-01-28, page: /videos)
```

### 3. Cursor File

**File:** `.feedback-cursor` (gitignored)

**Contents:** Single line with the ISO timestamp of the most recent feedback item fetched.

```
2025-01-28T14:30:00Z
```

On each run, the script passes `&after=<cursor>` to only get new items. If cursor file doesn't exist, fetch all `status=new` items.

**Note:** This requires adding an `after` query parameter to the backend's `/api/admin/feedback` endpoint (filter by `created_at > ?`).

### 4. Gitignore Additions

Add to `.gitignore`:
```
.env.feedback
.feedback-cursor
```

## Integration Options

### Option A: Ralph calls it (recommended)

Add to `ralph.py` before the claude invocation:
```python
# Fetch new feedback before each iteration
subprocess.run(["bash", "scripts/fetch-feedback.sh"], cwd=project_dir)
```

**Pros:** Simple, runs at the right time (before each Ralph iteration), no extra infrastructure.

**Cons:** Requires ralph.py modification.

### Option B: Cron job / systemd timer

Run `fetch-feedback.sh` on a schedule (e.g., every 15 minutes).

**Pros:** Decoupled from Ralph, feedback arrives even when Ralph isn't running.

**Cons:** Extra system configuration, feedback may pile up, timing mismatch.

### Option C: Git pre-push hook

Run fetch before pushing so deployed code includes latest feedback.

**Pros:** Ties feedback to deploy cycle.

**Cons:** Only runs on push, not on Ralph iterations.

**Recommendation:** Option A. It's the simplest path to "fetch feedback, Ralph reads it, Ralph acts on it, Ralph pushes, auto-deploy picks it up."

## Backend Changes Required

Add `after` query parameter to `GET /api/admin/feedback`:

```go
// In the feedback list handler
if after := r.URL.Query().Get("after"); after != "" {
    // Add WHERE created_at > ? to the query
}
```

## Security Considerations

1. **Session ID rotation:** The API session ID will expire. The script should detect 401 responses and print a clear message to update `.env.feedback`.
2. **No secrets in git:** `.env.feedback` is gitignored. If using `secrets.enc.yaml`, only the encrypted form is committed.
3. **Rate limiting:** The admin endpoint has rate limiting. The script makes one call per Ralph iteration, well within limits.

## Testing

1. **Manual test:** Run `bash scripts/fetch-feedback.sh` and verify FEEDBACK.md is created/updated
2. **No-new-feedback test:** Run twice, second run should exit 1 with no changes to FEEDBACK.md
3. **Auth failure test:** Use invalid session ID, verify clear error message
4. **Cursor test:** Verify `.feedback-cursor` is updated and subsequent runs only get newer items

## Files to Create/Modify

| File | Action |
|------|--------|
| `scripts/fetch-feedback.sh` | Create |
| `.env.feedback` | Create (manual, gitignored) |
| `.gitignore` | Add entries |
| `backend/main.go` | Add `after` param to feedback list endpoint |
| `ralph.py` | Add fetch-feedback call before iteration |
| `.feedback-cursor` | Auto-created by script (gitignored) |
