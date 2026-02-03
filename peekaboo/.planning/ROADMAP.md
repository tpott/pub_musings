# Roadmap: Peekaboo

## Overview

Peekaboo delivers a voice-activated content viewer where a child says "show me a cat" and immediately sees and hears that thing. The roadmap builds from content storage foundation through audio capture, real-time transcription, LLM intent detection, and polished UI, culminating in admin tools and production deployment. Each phase proves a component works before integrating with the next, surfacing the highest-risk element (child speech + LLM intent) as early as possible while having infrastructure to test it.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Content Foundation** - SQLite schema, age encryption, basic content API
- [ ] **Phase 2: Audio Pipeline** - Browser audio capture, whisper-server integration
- [ ] **Phase 3: Real-Time Streaming** - WebSocket layer for live transcription display
- [ ] **Phase 4: Intent & Content Loop** - LLM intent detection, content display, the magic
- [ ] **Phase 5: Frontend Polish** - Mic toggle, visual indicators, toddler-friendly UX
- [ ] **Phase 6: Admin & Deployment** - Admin API, content management, production deployment

## Phase Details

### Phase 1: Content Foundation
**Goal**: Content can be stored encrypted and served decrypted on demand
**Depends on**: Nothing (first phase)
**Requirements**: DATA-01, DATA-02, DATA-03
**Success Criteria** (what must be TRUE):
  1. SQLite database stores concept-to-media mappings (e.g., "cat" -> cat.jpg.age)
  2. Media files are encrypted with age (X25519 + ChaCha20-Poly1305)
  3. Content API returns decrypted media when requested by concept name
  4. Test content (5+ concepts) can be manually loaded and served
**Plans**: TBD

Plans:
- [ ] 01-01: TBD
- [ ] 01-02: TBD

### Phase 2: Audio Pipeline
**Goal**: Browser captures audio and whisper-server returns transcriptions
**Depends on**: Phase 1
**Requirements**: VOICE-02, VOICE-04
**Success Criteria** (what must be TRUE):
  1. Browser captures audio via MediaRecorder (WebM format)
  2. Audio chunks are sent to Go backend via HTTP POST
  3. Backend forwards audio to whisper-server and receives transcription
  4. End-to-end test: speak into browser, see transcription text returned
**Plans**: TBD

Plans:
- [ ] 02-01: TBD
- [ ] 02-02: TBD

### Phase 3: Real-Time Streaming
**Goal**: User sees words appear as they speak (live transcription display)
**Depends on**: Phase 2
**Requirements**: UX-01
**Success Criteria** (what must be TRUE):
  1. WebSocket connection established between browser and Go backend
  2. Audio streams continuously while speaking (not chunked uploads)
  3. Transcription appears in scrolling dialog box in real-time
  4. Latency from speech to display is under 1 second
**Plans**: TBD

Plans:
- [ ] 03-01: TBD
- [ ] 03-02: TBD

### Phase 4: Intent & Content Loop
**Goal**: Say "show me a cat" and see a cat photo with optional audio
**Depends on**: Phase 3
**Requirements**: VOICE-03, DISP-01, DISP-02, DISP-03
**Success Criteria** (what must be TRUE):
  1. LLM (Anthropic Claude) detects intent from transcription stream without wake word
  2. Matching concept triggers full-screen photo display
  3. Audio clip plays automatically if available for the concept
  4. End-to-end latency (speech to content visible) is under 2 seconds
  5. System handles noisy/garbled child speech gracefully (phonetic matching)
**Plans**: TBD

Plans:
- [ ] 04-01: TBD
- [ ] 04-02: TBD
- [ ] 04-03: TBD

### Phase 5: Frontend Polish
**Goal**: Parents can control the mic and see clear listening state
**Depends on**: Phase 4
**Requirements**: VOICE-01
**Success Criteria** (what must be TRUE):
  1. Mic toggle button clearly indicates listening state (red = listening)
  2. Touch targets are large enough for toddler motor skills (48px+)
  3. Mic state persists correctly across content displays
  4. Parent can pause listening at any time with single tap
**Plans**: TBD

Plans:
- [ ] 05-01: TBD

### Phase 6: Admin & Deployment
**Goal**: Content can be managed and app runs in production
**Depends on**: Phase 5
**Requirements**: ADMIN-01, ADMIN-02, DEPLOY-01, DEPLOY-02, DEPLOY-03, DEPLOY-04, DEPLOY-05
**Success Criteria** (what must be TRUE):
  1. Admin API accepts new content with automatic age encryption
  2. Admin API supports CRUD operations for concepts and media
  3. App accessible via cloudflared tunnel with HTTPS
  4. Go backend runs as systemd service with auto-restart
  5. Deployments triggered via webhook-deployer from git push
**Plans**: TBD

Plans:
- [ ] 06-01: TBD
- [ ] 06-02: TBD
- [ ] 06-03: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5 -> 6

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Content Foundation | 0/2 | Not started | - |
| 2. Audio Pipeline | 0/2 | Not started | - |
| 3. Real-Time Streaming | 0/2 | Not started | - |
| 4. Intent & Content Loop | 0/3 | Not started | - |
| 5. Frontend Polish | 0/1 | Not started | - |
| 6. Admin & Deployment | 0/3 | Not started | - |

---
*Roadmap created: 2026-02-02*
*Total requirements: 18 | Total phases: 6*
