# Status

This file tracks high level progress on the peekaboo project.

## Current State

Production-ready voice-controlled web app for children. 162 tasks completed.

### Architecture
- **Go backend** with SQLite, WebSocket audio streaming, age encryption
- **Astro frontend** with TypeScript, 269 unit tests, 22+ E2E tests
- **External services**: whisper-server (STT), Anthropic/OpenAI (intent), optional Piper (TTS)

### Key Features
- Full voice flow: record → transcribe → intent → media display
- WebSocket streaming (default) with continuous listening mode
- Day/night theme (light/dark/auto) with warm neutral palette
- Feedback form with database persistence and rate limiting
- Piper TTS integration (optional)
- Accessibility: ARIA labels, skip-to-content, keyboard navigation, screen reader support
- Security: CSP headers, path traversal defense, URL validation, rate limiting, CORS
- Structured logging, health probes, graceful shutdown

### Documentation
- docs/API.md, docs/DEPLOY.md, docs/SECURITY.md, docs/TROUBLESHOOTING.md, docs/PERFORMANCE.md
- specs/ directory with architecture, WebSocket protocol, continuous listening, feedback, Piper TTS, Bazel research

### Quality
- Linting: gofmt, go vet, golangci-lint, npm build, filesize lint (scripts/lint.sh)
- Pre-commit hook runs lint + tests for changed projects
- Test organization: files split to stay under 500 lines

## Last Completed

- Tasks 157-162 (2026-02-06): Security hardening, XSS fix, memory leak fixes, debug logging, E2E theme tests
  - Task 157: Limit LLM error response body reads with io.LimitReader (10KB cap)
  - Task 158: Replace innerHTML with DOM methods in showBrowserSupportError
  - Task 159: Track and clear success timeout in FeedbackButton
  - Task 160: Add E2E test for theme toggle (3 tests: cycling, persistence, localStorage)
  - Task 161: Add PeekabooFlow cleanup on page unload (pagehide → destroy)
  - Task 162: Add debug logging to silent catch blocks in websocket-audio.ts and errors.ts

## Milestone History

- Tasks 1-14 (2026-02-03/04): Initial scaffold, media, DB, APIs, frontend flow, E2E
- Tasks 15-25 (2026-02-04): Deployment, env config, CORS, SQLite optimization
- Tasks 26-59 (2026-02-04): Security, validation, health checks, rate limiting, accessibility
- Tasks 60-81 (2026-02-04): Piper TTS, WebSocket audio streaming, comprehensive testing
- Tasks 82-102 (2026-02-04/05): Performance tuning, error handling, continuous listening UX
- Tasks 103-123 (2026-02-05): Code organization, feedback, theme, linting
- Tasks 124-127 (2026-02-05): Security hardening, code cleanup, docs, CI guard
- Tasks 128-131 (2026-02-05): Server timeouts, XSS fix, doc fixes, event listener cleanup
- Task 132 (2026-02-05): Timer leak fixes (error timeout, reconnect timeout)
- Task 133 (2026-02-05): Unchecked ResponseWriter.Write error handling
- Task 134 (2026-02-05): WebSocket magic numbers → named constants
- Task 135 (2026-02-05): Failing test proving WebM container corruption on buffer split
- Task 136 (2026-02-05): Failing test proving whisper verbose_json word data is discarded
- Task 137 (2026-02-06): Fix WebM container corruption with EBML init segment caching
- Task 138 (2026-02-06): Parse full whisper verbose_json with word-level timing
- Task 139 (2026-02-06): Framed audio protocol with 12-byte headers and timestamp mapping
- Task 140 (2026-02-06): LLM boundary detection with tool_choice:any and three tools
- Task 141 (2026-02-06): Audio buffer trimming at Cluster boundaries + transcript accumulation
- Task 142 (2026-02-06): TTS tool execution and multi-tool sequential processing
- Task 143 (2026-02-06): Whisper VAD integration for silence-based triggering
- Task 144 (2026-02-06): E2E endurance test for continuous listening
- Task 145 (2026-02-06): Split oversized websocket_llm_test.go into two files
- Task 146 (2026-02-06): Split 5 remaining oversized test files (backend + frontend)
- Task 147 (2026-02-06): Update audio-timing.md spec status to Implemented
- Task 148 (2026-02-06): Update API.md WebSocket protocol documentation
- Task 149 (2026-02-06): Update outdated documentation references
- Task 150 (2026-02-06): Fix PeekabooFlow.destroy() resource cleanup gaps
- Task 151 (2026-02-06): Fix response body leak in fetchWithRetry on retryable status codes
- Task 152 (2026-02-06): Fix InstructionEndWordIdx zero-value ambiguity in LLM tool parsing
- Task 153 (2026-02-06): Fix WebSocket reconnection bug + reconnect/ping/message tests
- Task 154 (2026-02-06): Add media load error handlers + action loop context cancellation
- Task 155 (2026-02-06): Wrap handleWsMedia in try-catch to prevent stuck state on display error
- Task 156 (2026-02-06): Add tests for untested PeekabooFlow code paths
- Tasks 157-162 (2026-02-06): Security hardening batch + E2E theme test coverage
