# Project Research Summary

**Project:** Peekaboo - Voice-activated content viewer for kids
**Domain:** Voice-activated educational/entertainment application for toddlers
**Researched:** 2026-02-02
**Confidence:** MEDIUM-HIGH

## Executive Summary

Peekaboo is a voice-activated content viewer targeting toddlers (2-5 years), using continuous speech-to-text with LLM-based intent detection to display curated photos/videos/audio without wake words. Research reveals this domain has specific challenges: child speech recognition suffers 37-87% error rates compared to adult speech, requiring noise-tolerant design at every layer. The recommended approach leverages proven patterns from the existing subtitler project (Go backend, age encryption, whisper.cpp STT) while adding real-time WebSocket streaming and dual LLM provider support (Anthropic/OpenAI) for robust intent detection.

The critical success factors are: (1) designing for degraded ASR accuracy with phonetic matching and vocabulary priors, (2) maintaining sub-2-second latency budgets through streaming architectures, and (3) implementing COPPA-compliant privacy (no voice recording persistence, all content curated locally). The architecture follows a proven pattern: browser audio capture via MediaRecorder, WebSocket streaming to Go backend, whisper.cpp service for STT, LLM for intent detection, and encrypted local content storage. Key risks include mobile browser permission complexity, WebSocket reconnection storms, and latency accumulation across the pipeline.

Research confidence is HIGH for stack choices (leveraging subtitler patterns) and privacy requirements (COPPA is well-documented), MEDIUM for child-specific UX patterns (less research available), and requires ongoing validation for the novel "no wake word" LLM intent detection approach.

## Key Findings

### Recommended Stack

The technology stack builds on proven patterns from the subtitler project while adding real-time streaming capabilities. Go 1.24.x provides runtime consistency with existing infrastructure, enabling official LLM SDKs (anthropic-sdk-go, openai-go) for intent detection. The frontend uses Next.js 15+ with React 19 for real-time state management, departing from subtitler's Astro choice due to project requirements and WebSocket needs.

**Core technologies:**
- **Go 1.24.x + coder/websocket**: Backend runtime with modern WebSocket library (full context support, actively maintained)
- **whisper.cpp server**: STT service via HTTP API, proven in subtitler, supports VAD and multiple models
- **filippo.io/age encryption**: X25519 + ChaCha20-Poly1305 for media files, direct reuse from subtitler
- **Anthropic/OpenAI official SDKs**: Dual provider support for LLM intent detection with streaming
- **Next.js 15+ + React 19**: Frontend framework for real-time WebSocket client and responsive UI
- **SQLite + mattn/go-sqlite3**: Content catalog with proven write performance

**Critical decisions:**
- coder/websocket over gorilla/websocket (better context support, actively maintained)
- Official LLM SDKs over community alternatives (priority support for new features)
- whisper.cpp HTTP over WhisperLive (simpler integration, proven pattern)
- Browser-native MediaRecorder over Web Speech API (privacy: no Google servers)

### Expected Features

Research identifies clear feature priorities for a toddler-focused voice app, with emphasis on safety, simplicity, and immediate feedback.

**Must have (table stakes):**
- Visible listening indicator (red pulsing) - 93% of parents require knowing when mic is active
- Mic toggle button - explicit parental control, COPPA compliance essential
- Immediate visual feedback - toddlers need <2s response for "magical" experience
- Audio feedback chimes - confirmation sounds when system responds
- Large touch targets (48px+) - toddler motor skills require oversized UI
- Curated content only - zero user-generated or API-sourced content for safety
- No ads/in-app purchases - table stakes for kids apps
- Simple error recovery - 50% ASR failure rate for children requires gentle retry UX

**Should have (differentiators):**
- No wake word required - LLM judges intent from context ("show me a cat" just works)
- Continuous streaming STT - low-latency response with words appearing as spoken
- Real-time transcription display - scrolling dialog box shows speech is understood
- Context-aware intent detection - handles natural speech like "show her a dog"
- Offline-first architecture - all content local, works without internet
- Dual LLM provider support - flexibility between Anthropic/OpenAI

**Defer (v2+):**
- Text-to-speech responses - explicitly deferred per PROJECT.md
- Weather/time/calculator tools - out of scope, adds complexity
- API-sourced content - requires content moderation, latency concerns
- Multiple content items per concept - start with 1:1 mapping
- Video playback - begin with photos + audio clips

**Anti-features (do not build):**
- Cloud-stored voice recordings - FTC fined Amazon $25M for this with kids
- Rigid wake word detection - unnatural for toddlers
- Long session design - toddler attention spans are 3-8 minutes
- Profile-based personalization - COPPA compliance burden

### Architecture Approach

The architecture uses WebSocket-based streaming with clear component boundaries. Browser captures audio via MediaRecorder, streams chunks to Go backend via WebSocket, backend forwards to whisper.cpp HTTP service for transcription, feeds transcription to LLM for intent detection, queries SQLite for content, decrypts .age files on-demand, and returns results to frontend. This pattern achieves target latency of 1.6-2s end-to-end.

**Major components:**
1. **React Frontend** - Audio capture via Web Audio API/MediaRecorder, WebSocket client, content display with custom hooks for state management
2. **Go Backend** - WebSocket server (coder/websocket), audio forwarding to Whisper, LLM orchestration with streaming, content API, age encryption/decryption
3. **Whisper Server** - Speech-to-text via whisper.cpp HTTP server, runs on Mac Mini as separate service (proven from subtitler)
4. **SQLite + .age files** - Content catalog with encrypted media storage, decrypt-on-demand pattern
5. **LLM Provider** - Intent detection via official Anthropic/OpenAI SDKs with abstraction layer for dual provider support

**Key patterns:**
- Message-based WebSocket protocol with JSON for control, binary for audio
- Graceful degradation for component failures (Whisper down, LLM slow)
- Debounced LLM calls (500ms after last transcription update, not every chunk)
- Streaming decryption via DecryptReader, not DecryptToFile
- Client-side VAD to reduce battery drain and network usage

**Latency budget (target <2s total):**
- Audio capture + VAD: 200ms
- Network to Whisper: 100ms
- Whisper processing: 500ms (fast model, small chunks)
- LLM inference: 500ms (streaming response)
- Content decryption: 200ms (cache common content)
- Content display: 100ms

### Critical Pitfalls

Research reveals several domain-specific pitfalls that require design-time mitigation, not just implementation fixes.

1. **Child Speech Recognition Accuracy Cliff** - ASR models achieve 2.89% WER on adults but spike to 38-87% WER on toddlers. Design LLM prompts for noise tolerance, implement phonetic similarity matching ("tat" -> "cat"), use vocabulary priors from concept database, test with actual children's voices. (HIGH confidence, multiple academic papers)

2. **Mobile Browser Audio Permissions Hell** - getUserMedia fails silently across platforms due to HTTPS requirements, iOS user gesture timing, OS-level permissions, and Permissions-Policy headers. Enforce HTTPS from day one, implement pre-flight permission checks, handle all error states explicitly, test on actual mobile devices. (HIGH confidence, MDN + developer reports)

3. **WebSocket Reconnection Storm on Deployment** - Server restart drops all connections, clients retry simultaneously, cascading failures ensue. Implement exponential backoff with jitter upfront, not as an afterthought. (HIGH confidence, well-documented pattern)

4. **STT Streaming Latency Accumulation** - Multiple pipeline stages compound latency (audio buffer, network, Whisper chunks, VAD delays, LLM inference). Vendor "sub-300ms" claims exclude critical overhead; real-world latency easily reaches 2-5s. Measure end-to-end, use small audio chunks, stream partial transcripts, aggressive endpointing. (HIGH confidence, multiple benchmarks)

5. **LLM Intent Detection on Noisy Transcription** - Garbled child speech plus background noise creates false positives/negatives. Design prompts explicitly for noisy child transcription, provide concept vocabulary to LLM, require confidence thresholds, implement "I heard X but I'm not sure" responses. (MEDIUM confidence, general LLM patterns)

## Implications for Roadmap

Based on research, the recommended phase structure prioritizes proving the core voice-to-content loop before adding polish or administrative features. Dependencies flow from foundation (content storage) through integration (Whisper, WebSocket) to intelligence (LLM) to UX refinement.

### Phase 1: Content Foundation & Basic Backend
**Rationale:** Must have content infrastructure before anything else works. SQLite schema and age encryption are proven patterns from subtitler, minimizing risk. Basic HTTP server enables early testing without WebSocket complexity.

**Delivers:** SQLite database schema, age encryption module (copied from subtitler), basic Go HTTP server with health checks, manual content population capability, content serving API with decryption.

**Addresses:** Curated content requirement (FEATURES.md table stake), offline-first architecture (FEATURES.md differentiator).

**Avoids:** Age encryption performance pitfall (implement streaming decryption from start).

**Research flag:** Low - uses proven subtitler patterns, standard Go/SQLite.

### Phase 2: Whisper Integration & Audio Pipeline
**Rationale:** STT is the core capability; must prove it works with real audio before adding WebSocket complexity. Audio format conversion (webm -> WAV) and Whisper HTTP client are testable independently.

**Delivers:** Audio format conversion (ffmpeg), Whisper HTTP client, test harness for audio upload/transcription, VAD integration (client-side Silero recommended).

**Uses:** whisper.cpp server, ffmpeg for transcoding.

**Avoids:** STT latency accumulation (benchmark early, optimize chunk sizes), VAD false positives (use Silero not WebRTC), Whisper buffer misconfiguration.

**Research flag:** Medium - whisper.cpp patterns documented but child-specific tuning needs experimentation.

### Phase 3: WebSocket Real-Time Pipeline
**Rationale:** Once Whisper works via HTTP, add streaming layer. This proves the audio capture -> transcription -> display loop before introducing LLM complexity.

**Delivers:** Go WebSocket server (coder/websocket), React audio capture (getUserMedia + MediaRecorder), React WebSocket client, end-to-end test (speak -> see transcription).

**Uses:** coder/websocket (STACK.md), Web Audio API + MediaRecorder.

**Implements:** WebSocket architecture component (ARCHITECTURE.md), message-based protocol.

**Avoids:** Mobile permissions hell (implement HTTPS + pre-flight checks), reconnection storm (exponential backoff with jitter), React audio state desync (functional setState, event-driven).

**Research flag:** Medium - WebSocket patterns standard but mobile audio capture has platform quirks needing validation.

### Phase 4: LLM Intent Detection & Content Serving
**Rationale:** With working transcription stream, add intelligence layer. LLM provider abstraction enables fallback between Anthropic/OpenAI. This completes the core voice-to-content loop.

**Delivers:** LLM provider abstraction (interface for Anthropic + OpenAI), intent detection logic with prompt engineering, content lookup from SQLite by concept, debounced LLM calls (500ms after transcription update).

**Uses:** anthropic-sdk-go + openai-go official SDKs (STACK.md).

**Addresses:** No wake word requirement (FEATURES.md differentiator), context-aware intent (FEATURES.md differentiator).

**Avoids:** LLM intent on noisy transcription (design prompts for child speech errors, provide vocabulary), child speech accuracy cliff (phonetic matching, vocabulary priors).

**Research flag:** High - LLM prompt engineering for noisy child speech is novel, needs significant iteration and testing with real toddler audio.

### Phase 5: Frontend UX & Visual Feedback
**Rationale:** With functional pipeline, add polish that makes it usable for toddlers. Visual feedback is table stakes but can be iterated after core works.

**Delivers:** Transcription display (scrolling dialog box), content display area (photo/video/audio player), mic toggle UI (red indicator when listening), audio feedback chimes, error states with child-friendly messages.

**Addresses:** Visible listening indicator (FEATURES.md table stake), immediate visual feedback (FEATURES.md table stake), large touch targets (FEATURES.md table stake).

**Avoids:** Missing visual feedback pitfall, toddler-inappropriate interactions, React audio state desynchronization.

**Research flag:** Low - UX patterns well-documented, mainly implementation work.

### Phase 6: Admin Tools & Content Management
**Rationale:** Can be developed in parallel with Phase 5. Admin functionality doesn't block user-facing features. Python REPL helpers simplify content curation workflow.

**Delivers:** Admin API endpoints (CRUD for concepts and media), Python REPL helpers for local content review, upload scripts with age encryption.

**Addresses:** Content curation pipeline (FEATURES.md differentiator).

**Research flag:** Low - standard CRUD API patterns.

### Phase Ordering Rationale

- **Foundation first (Phase 1)**: Content storage has no external dependencies, provides test data for subsequent phases
- **Whisper before WebSocket (Phase 2 before 3)**: Simpler to test STT via HTTP upload than debug WebSocket + audio capture + STT simultaneously
- **Streaming before intelligence (Phase 3 before 4)**: Proves real-time transcription before adding LLM complexity; enables testing LLM prompts with known-good transcriptions
- **Core loop before polish (Phases 1-4 before 5)**: Visual feedback is important but can iterate after proving voice-to-content works
- **Admin parallel to UX (Phase 6)**: Non-blocking; can manually populate DB while building phases 1-5

This ordering minimizes integration risk by validating each component independently before combining, and surfaces the highest-risk element (child speech + LLM intent detection) as early as possible while having infrastructure to test it.

### Research Flags

**Phases needing deeper research during planning:**
- **Phase 4 (LLM Intent):** Novel approach combining noisy child ASR with LLM intent detection; prompt engineering needs significant experimentation; consider `/gsd:research-phase` for LLM prompt patterns and child speech error mitigation strategies
- **Phase 3 (WebSocket on mobile):** Mobile browser audio capture has platform-specific quirks; may need targeted research for iOS Safari vs Chrome permission flows

**Phases with standard patterns (skip research-phase):**
- **Phase 1 (Content Foundation):** Direct reuse of subtitler encryption and SQLite patterns
- **Phase 2 (Whisper):** whisper.cpp server integration proven in subtitler
- **Phase 5 (Frontend UX):** Standard React patterns, well-documented child UX guidelines
- **Phase 6 (Admin Tools):** Basic CRUD API, standard patterns

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Leverages proven subtitler patterns (Go, age, whisper.cpp); official LLM SDKs well-documented; coder/websocket actively maintained |
| Features | MEDIUM | Table stakes confirmed by multiple sources; differentiators (no wake word, LLM intent) are novel and need validation; COPPA requirements are clear |
| Architecture | MEDIUM-HIGH | WebSocket streaming patterns standard; component boundaries proven; latency budget realistic but needs measurement; child-specific optimizations less documented |
| Pitfalls | MEDIUM-HIGH | Child ASR accuracy cliff confirmed by academic research; mobile permissions and WebSocket patterns well-documented; LLM prompt engineering for noisy child speech needs validation |

**Overall confidence:** MEDIUM-HIGH

Research provides strong foundation for stack and architecture decisions (leveraging subtitler patterns significantly de-risks). Primary uncertainty is in the novel combination of child speech ASR + LLM intent detection without wake words, which requires experimentation during Phase 4.

### Gaps to Address

Areas where research was inconclusive or needs validation during implementation:

- **Child speech prompt engineering:** LLM prompt patterns for handling phonetically similar errors ("tat" -> "cat") and child-specific disfluencies need experimentation with real toddler audio samples. Plan to iterate extensively during Phase 4.

- **Mobile browser audio quirks:** iOS Safari vs Chrome permission flows documented but specific to browser versions; plan to test on actual devices early in Phase 3, may discover platform-specific workarounds needed.

- **Latency optimization:** 2s budget is achievable in theory but requires end-to-end measurement with actual deployment topology (Mac Mini whisper location, network latency). Benchmark early in Phase 2-3, adjust model/chunk sizes as needed.

- **VAD tuning for home environment:** Silero VAD shows good benchmarks but home environments (TV, toys, siblings) may require sensitivity tuning. Plan to collect real-world false positive data during testing.

- **Content decryption caching strategy:** Whether to cache decrypted content in memory vs. decrypt-on-demand depends on content library size (unknown at research phase). Start with decrypt-on-demand, add caching if latency issues emerge.

- **LLM provider failover logic:** How to handle one provider down or rate-limited needs design during Phase 4; simple round-robin vs. intelligent fallback based on response quality.

## Sources

### Primary (HIGH confidence)
- Subtitler project specifications: encryption.md, backend.md, stt-providers.md - proven patterns for age encryption, whisper.cpp integration, SQLite
- Official documentation: [coder/websocket](https://github.com/coder/websocket), [anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go), [openai-go](https://github.com/openai/openai-go), [whisper.cpp](https://github.com/ggml-org/whisper.cpp)
- [MDN getUserMedia()](https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getUserMedia) - browser audio capture
- [FTC COPPA FAQ](https://www.ftc.gov/business-guidance/resources/complying-coppa-frequently-asked-questions) - official compliance requirements

### Secondary (MEDIUM confidence)
- Academic research: [Transfer Learning from Adult to Children for Speech Recognition (PMC)](https://pmc.ncbi.nlm.nih.gov/articles/PMC7199459/), [Improving End-to-End Models for Children's Speech Recognition](https://www.mdpi.com/2076-3417/14/6/2353) - child ASR accuracy challenges
- Industry benchmarks: [Picovoice VAD Comparison 2026](https://picovoice.ai/blog/best-voice-activity-detection-vad/), [Deepgram STT Latency Guide](https://deepgram.com/learn/understanding-and-reducing-latency-in-speech-to-text-apis)
- Developer resources: [WebSocket Reconnection Logic (OneUptime)](https://oneuptime.com/blog/post/2026-01-24-websocket-reconnection-logic/view), [Common getUserMedia Errors (AddPipe)](https://blog.addpipe.com/common-getusermedia-errors/)
- Child UX patterns: [Futurice Voice Services for Kids](https://www.futurice.com/blog/how-to-design-great-voice-services-for-kids), [UX for Kids Gen Alpha (BitsKingdom)](https://bitskingdom.com/blog/ux-for-kids-gen-alpha-toddlers/)

### Tertiary (LOW confidence - needs validation)
- WhisperLive streaming alternative - documented but not production-tested
- Next.js 16 status - mentioned but verify before adopting
- Exact child WER percentages vary by study methodology - use as directional guidance

---
*Research completed: 2026-02-02*
*Ready for roadmap: yes*
