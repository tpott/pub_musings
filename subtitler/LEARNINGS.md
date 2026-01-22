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
