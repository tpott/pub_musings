# Requirements: Peekaboo

**Defined:** 2026-02-02
**Core Value:** A child says "show me a [thing]" and immediately sees and hears that thing. The voice-to-visual loop must feel instant and magical.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Voice Pipeline

- [ ] **VOICE-01**: Mic toggle button with visual state indicator (red = listening)
- [ ] **VOICE-02**: Streaming STT via whisper-server (HTTP POST to separate service)
- [ ] **VOICE-03**: LLM intent detection using Anthropic Claude API (no wake word required)
- [ ] **VOICE-04**: Audio capture in browser via Web Audio API + MediaRecorder

### Content Display

- [ ] **DISP-01**: Full-screen photo display when content matched
- [ ] **DISP-02**: Audio clip playback with photos (optional per concept)
- [ ] **DISP-03**: Offline-first — all content stored locally encrypted

### Feedback UX

- [ ] **UX-01**: Real-time transcription display in scrolling dialog box

### Content Database

- [ ] **DATA-01**: SQLite database mapping concepts to media files
- [ ] **DATA-02**: Age encryption for all media files (.age extension)
- [ ] **DATA-03**: Decrypt-on-demand for serving content

### Admin

- [ ] **ADMIN-01**: Admin API endpoints for adding/managing content
- [ ] **ADMIN-02**: Content upload with automatic encryption

### Deployment

- [ ] **DEPLOY-01**: Cloudflared tunnel configuration
- [ ] **DEPLOY-02**: Caddy reverse proxy (static frontend + API proxy to backend)
- [ ] **DEPLOY-03**: systemd service for Go backend
- [ ] **DEPLOY-04**: webhook-deployer config for CI/CD
- [ ] **DEPLOY-05**: whisper-server running on Mac Mini host (accessible via 10.0.2.2)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Voice Pipeline

- **VOICE-05**: Dual LLM provider support (add OpenAI alongside Anthropic)
- **VOICE-06**: whisper-stream + WebSocket for lower latency STT

### Content Display

- **DISP-04**: Video playback support

### Feedback UX

- **UX-02**: Audio chimes (listening start, request understood)
- **UX-03**: Graceful error recovery UI ("try again" prompts)

### Admin

- **ADMIN-03**: Python REPL + helper scripts for local content review/upload

### Extended Features

- **EXT-01**: TTS responses for non-content queries
- **EXT-02**: Weather tool integration
- **EXT-03**: Time/date tool integration
- **EXT-04**: API-sourced content (Unsplash, Pexels)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Wake word detection | LLM judges intent from context — more natural for kids |
| User-generated content | Safety nightmare, moderation impossible, COPPA risk |
| Cloud-stored voice recordings | COPPA treats voice as PII — stream-process only |
| User accounts/profiles | No data collection on children, anonymous sessions |
| Mobile native app | Web-first approach |
| Gamification / points / badges | Adds complexity without core value |
| Continuous conversation mode | Privacy risk, each request independent |
| Child voice training | Requires storing voice samples, COPPA risk |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| VOICE-01 | Phase 5 | Pending |
| VOICE-02 | Phase 2 | Pending |
| VOICE-03 | Phase 4 | Pending |
| VOICE-04 | Phase 2 | Pending |
| DISP-01 | Phase 4 | Pending |
| DISP-02 | Phase 4 | Pending |
| DISP-03 | Phase 4 | Pending |
| UX-01 | Phase 3 | Pending |
| DATA-01 | Phase 1 | Pending |
| DATA-02 | Phase 1 | Pending |
| DATA-03 | Phase 1 | Pending |
| ADMIN-01 | Phase 6 | Pending |
| ADMIN-02 | Phase 6 | Pending |
| DEPLOY-01 | Phase 6 | Pending |
| DEPLOY-02 | Phase 6 | Pending |
| DEPLOY-03 | Phase 6 | Pending |
| DEPLOY-04 | Phase 6 | Pending |
| DEPLOY-05 | Phase 6 | Pending |

**Coverage:**
- v1 requirements: 18 total
- Mapped to phases: 18
- Unmapped: 0

---
*Requirements defined: 2026-02-02*
*Last updated: 2026-02-02 after roadmap creation*
