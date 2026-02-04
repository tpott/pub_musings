# Status

This file tracks high level progress on the peekaboo project.

## Current State

- **Go backend fully wired** - backend/ with main.go, all API handlers, database init, static files, CORS via ALLOWED_ORIGIN env
- **Astro frontend scaffolded** - mobile-first layout with mic button and media display
- **Media assets sourced** - 6 animals with CC0 photos and audio via scripts/source-media.sh
- **SQLite database** - db package with concepts/media_sets tables, WAL mode, busy timeout, tests pass
- **Age encryption** - crypto package with EncryptFile/DecryptFile/DecryptReader, encrypted media serving via EncryptedFileServer
- **Whisper API** - api/transcribe.go forwards audio to whisper-server, returns transcript
- **LLM Intent API** - api/intent.go uses llm.Provider interface for intent extraction
- **LLM Provider abstraction** - llm package supports Anthropic and OpenAI; main.go creates provider from env
- **Media lookup API** - api/media.go returns random media set for a concept, with input validation
- **Environment config** - .env.example documents all required environment variables
- **Frontend mic recording** - MicButton with MediaRecorder, sends audio to /api/transcribe
- **Frontend media display** - MediaDisplay class renders images/videos, auto-plays audio
- **Full frontend flow** - PeekabooFlow orchestrates: record -> transcribe -> intent -> media -> display
- **Test fixtures** - tests/fixtures/ with CC0 mock media and synthetic audio for e2e tests
- **Playwright e2e tests** - 2 tests verify full voice-to-media flow and error handling
- **Sops encryption** - secrets.enc.yaml with age encryption, docs/DEPLOY.md documents decrypt process
- **Deployment ready** - webhook-deployer scripts, systemd service, Caddy config documented
- **Piper TTS spec** - specs/piper.md documents installation, voice selection, HTTP API
- **Accessibility** - ARIA labels on mic button, aria-live region for media display, screen reader support
- **API documentation** - docs/API.md documents all backend endpoints with curl examples
- **Health probes** - Kubernetes-style /health/live and /health/ready endpoints
- **Input validation** - Minimum audio file size (1KB) for transcription

## Last Completed

- Task 32: Add minimum audio file size validation (1KB) to transcribe endpoint (2026-02-04)
- Task 31: Add liveness and readiness health probes with database connectivity check (2026-02-04)
- Task 30: Create docs/API.md with comprehensive endpoint documentation including curl examples (2026-02-04)
- Task 29: Add accessibility features - ARIA labels on mic button, role=region and aria-live=polite on media display (2026-02-04)
- Task 28: Sanitize error messages in API responses to prevent info leakage (2026-02-04)
- Task 27: api/media_test.go was already complete (2026-02-04)
- FEEDBACK: Move backend code to backend/ subdirectory (2026-02-04)
- FEEDBACK: Add exponential backoff to ralph.py for API server errors (2026-02-04)
- Task 26: Add input validation for concept ID in media API (2026-02-04)
- Task 25: Create .env.example file with documented environment variables (2026-02-04)
- Task 24: Optimize SQLite configuration with WAL mode, busy timeout, connection limits (2026-02-04)
- Task 23: Configure CORS properly with ALLOWED_ORIGIN env var (2026-02-04)
- Task 22: Integrate media encryption - encrypted .age files served with on-demand decryption (2026-02-04)
- Task 21: Wire up LLM provider abstraction - api/intent.go now uses llm.Provider (2026-02-04)
- Task 20: Wire up backend main.go with handlers, DB init, static files, CORS (2026-02-04)
- Task 18: Add OpenAI function calling support (2026-02-04)
- Task 17: Research piper TTS deployment (2026-02-04)
- Task 19: Deploy peekaboo via webhook-deployer (2026-02-04)
- Task 16: Add webhook-deployer config (2026-02-04) - config already existed, completed with task 19
- Task 15: Setup sops for env var encryption (2026-02-04)
- Task 14: Create Playwright e2e test with mocked APIs (2026-02-04)
- Task 13: Create test fixtures for Playwright e2e tests (2026-02-04)
- Task 12: Wire up full frontend flow with state management (2026-02-04)
- Task 11: Frontend media display with image/video/audio support (2026-02-04)
- Task 10: Frontend microphone recording with MediaRecorder (2026-02-04)
- Task 9: Media lookup API GET /api/media/{concept} (2026-02-04)
- Task 8: LLM intent recognition with Anthropic tool calls (2026-02-04)
- Task 7: Whisper-server integration (2026-02-03)
- Task 6: Age encryption (2026-02-03)
- Task 5: SQLite media database (2026-02-03)
- Task 4: source-media.sh script (2026-02-03)
- Task 3: .gitignore (2026-02-03)
- Task 2: Astro frontend scaffold (2026-02-03)
- Task 1: Go backend scaffold (2026-02-03)
