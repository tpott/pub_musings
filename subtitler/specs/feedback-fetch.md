# Feedback Fetch Pipeline

## Overview

A Python script and secrets management system that allows the development VM (claude4) to pull user feedback from production (`subtitler.pottingers.us`) and write it to `FEEDBACK.md`, creating a feedback loop for Ralph.

## Architecture

```
Production Server                 Dev VM (claude4)
┌─────────────────────┐          ┌─────────────────────────────┐
│ subtitler.pottingers │  HTTPS  │ scripts/fetch-feedback.py   │
│ .us                  │◄────────│                             │
│                      │         │ reads .env or env vars      │
│ GET /api/admin/      │         │ (or sops secrets.enc.yaml)  │
│   feedback           │─────────│ writes FEEDBACK.md          │
└─────────────────────┘         └─────────────────────────────┘
```

## Components

### 1. Secrets Resolution

Secrets are resolved in priority order (highest wins):

1. **Environment variables** - `PROD_HOST`, `API_SESSION_ID`
2. **`.env` file** - standard `KEY=VALUE` format in project root (gitignored)
3. **`secrets.enc.yaml`** - encrypted with age, decrypted via sops (git-tracked)

Example `.env` file:
```bash
# .env (gitignored)
PROD_HOST=https://subtitler.pottingers.us
API_SESSION_ID=<session cookie value>
```

Example `secrets.enc.yaml` (plaintext before encryption):
```yaml
prod_host: https://subtitler.pottingers.us
api_session_id: <session cookie value>
```

### 2. Fetch Script

**File:** `scripts/fetch-feedback.py`

**Behavior:**
1. Resolve secrets from env vars, `.env`, or `secrets.enc.yaml` (via sops)
2. Call `GET /api/admin/feedback?status=new&limit=50` with auth header
3. Track last-fetched timestamp in `.feedback-cursor` (gitignored) to avoid repeats
4. Format new feedback items as markdown
5. Write to `FEEDBACK.md` (append if exists, create if not)
6. Exit 0 if new feedback found, exit 1 if none, exit 2 on error

**Auth:** Uses `Authorization: Bearer <API_SESSION_ID>` header via urllib (no external dependencies).

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

### 4. Gitignore Entries

```
.env
.env.feedback
.feedback-cursor
```

## Integration

Ralph calls the script before each iteration via the `fetch_feedback()` function in `ralph.py`:

```python
result = subprocess.run(
    [sys.executable, str(script)],
    capture_output=True,
    text=True,
    timeout=30,
)
```

This is the simplest path: fetch feedback, Ralph reads it, Ralph acts on it, Ralph pushes, auto-deploy picks it up.

## Backend Changes Required

The `after` query parameter on `GET /api/admin/feedback` is already implemented (Task 341).

## Security Considerations

1. **Session ID rotation:** The API session ID will expire. The script detects 401 responses and prints a clear message to update credentials.
2. **No secrets in git:** `.env` is gitignored. If using `secrets.enc.yaml`, only the encrypted form is committed.
3. **Rate limiting:** The admin endpoint has rate limiting. The script makes one call per Ralph iteration, well within limits.

## Testing

1. **Help test:** Run `python3 scripts/fetch-feedback.py --help` to verify usage info
2. **Manual test:** Run `python3 scripts/fetch-feedback.py` and verify FEEDBACK.md is created/updated
3. **No-new-feedback test:** Run twice, second run should exit 1 with no changes to FEEDBACK.md
4. **Auth failure test:** Use invalid session ID, verify clear error message
5. **Cursor test:** Verify `.feedback-cursor` is updated and subsequent runs only get newer items

## Files

| File | Status |
|------|--------|
| `scripts/fetch-feedback.py` | Implemented |
| `.env` | Manual setup (gitignored) |
| `.gitignore` | Updated |
| `backend/main.go` | `after` param already implemented |
| `ralph.py` | Updated to call Python script |
| `.feedback-cursor` | Auto-created by script (gitignored) |
