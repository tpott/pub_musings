# Domain Pitfalls

**Domain:** Voice-activated content viewer for kids (Peekaboo)
**Researched:** 2026-02-02
**Overall Confidence:** MEDIUM (verified with multiple sources, some areas LOW)

---

## Critical Pitfalls

Mistakes that cause rewrites or major issues.

---

### Pitfall 1: Child Speech Recognition Accuracy Cliff

**What goes wrong:** Standard ASR models trained on adult speech produce catastrophically high error rates on toddler speech. A model achieving 2.89% Word Error Rate (WER) on adults can spike to 38-87% WER on children's speech, with younger children performing worst.

**Why it happens:** Children's speech differs fundamentally from adults in multiple dimensions:
- Higher fundamental frequency and formant frequencies (physiological)
- Greater variability in pitch and timing
- Incomplete linguistic knowledge leading to non-standard pronunciations
- Higher disfluency rates
- Age-related development (toddlers 12-36 months are "often beyond the scope of most ASR research")

**Consequences:**
- "Show me a cat" becomes "show me a hat" or unrecognizable
- LLM receives garbage transcription, cannot detect intent
- App appears broken or frustrating to children
- Parents lose trust in the system

**Prevention:**
1. **Accept degraded accuracy as baseline** - Design LLM prompts to be highly tolerant of transcription errors and phonetic variations
2. **Implement phonetic similarity matching** - "tat" should match "cat", "goggy" should match "doggy"
3. **Build a vocabulary prior** - The concept database (cat, dog, etc.) gives strong priors; bias toward known concepts
4. **Test with actual children** - Recorded adult speech testing is insufficient; capture real toddler audio for testing
5. **Consider Whisper fine-tuning** - Fine-tuning on children's speech data can significantly reduce WER (research shows VTLN and speed perturbation help)

**Detection (warning signs):**
- High transcription variance on same phrase
- LLM frequently responds with "I didn't understand"
- Parents repeat commands multiple times
- Test corpus of adult speech works but child speech fails

**Phase:** Should be addressed in Phase 1 (core audio pipeline) but requires ongoing refinement

**Confidence:** HIGH - Multiple academic papers confirm this is a fundamental challenge

**Sources:**
- [Transfer Learning from Adult to Children for Speech Recognition (PMC)](https://pmc.ncbi.nlm.nih.gov/articles/PMC7199459/)
- [Improving End-to-End Models for Children's Speech Recognition (MDPI)](https://www.mdpi.com/2076-3417/14/6/2353)
- [Causal analysis of ASR errors for children (ScienceDirect)](https://www.sciencedirect.com/science/article/pii/S0885230825000841)

---

### Pitfall 2: Mobile Browser Audio Capture Permissions Hell

**What goes wrong:** Audio capture fails silently or unpredictably across mobile browsers due to complex permission requirements, platform-specific quirks, and iframe restrictions.

**Why it happens:**
- **HTTPS required** - getUserMedia only works in secure contexts; http://localhost is an exception but production requires TLS
- **User gesture required on iOS** - Audio actions must be triggered by direct user click; delayed actions (e.g., after token fetch) fail
- **OS-level permissions** - macOS requires per-browser microphone permission; iOS requires Safari-specific settings
- **Permissions-Policy headers** - Cross-origin iframes block microphone by default (NotAllowedError)
- **Chrome on iOS** - May not properly trigger permission dialogs; Safari often required as fallback
- **Promise may never resolve** - User can ignore permission dialog indefinitely

**Consequences:**
- App appears broken on mobile devices
- "Works on my machine" during development, fails in production
- Difficult to debug because errors are inconsistent across platforms
- Users don't understand why app needs microphone permission

**Prevention:**
1. **Enforce HTTPS from day one** - Even in staging environments
2. **Pre-flight permission check** - Before attempting capture, check `navigator.permissions.query({name: 'microphone'})`
3. **Immediate action on user gesture** - Start audio immediately in click handler, not after async operations
4. **Handle all error states explicitly**:
   - `NotAllowedError` - User denied or policy blocked
   - `NotFoundError` - No microphone device
   - `NotReadableError` - Device in use or hardware error
5. **Clear UI for permission state** - Show what's happening, don't leave users guessing
6. **Test on actual mobile devices** - Emulators don't replicate permission behavior accurately
7. **Safari fallback messaging** - If Chrome on iOS fails, guide users to Safari

**Detection (warning signs):**
- High error rates in production analytics
- Support requests mentioning "microphone doesn't work"
- iOS users churning more than Android/desktop

**Phase:** Phase 1 (audio capture foundation)

**Confidence:** HIGH - MDN documentation and multiple developer reports confirm these issues

**Sources:**
- [getUserMedia() - MDN Web Docs](https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getUserMedia)
- [Common getUserMedia() Errors (AddPipe Blog)](https://blog.addpipe.com/common-getusermedia-errors/)
- [Permissions-Policy: microphone - MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Permissions-Policy/microphone)

---

### Pitfall 3: WebSocket Reconnection Storm on Deployment

**What goes wrong:** When the server restarts (deployment, crash, etc.), all connected clients attempt to reconnect simultaneously, overwhelming the server and causing cascading failures.

**Why it happens:**
- WebSockets are long-lived connections; a restart drops all of them at once
- Without backoff, clients retry immediately
- N clients x immediate retry = N simultaneous connection attempts
- Server can't handle the spike, rejects connections, clients retry again

**Consequences:**
- Deployment causes service outage
- Server crashes under reconnection load
- Users see extended "connecting..." states
- Cascading failures as retry storms compound

**Prevention:**
1. **Exponential backoff with jitter** - Not just exponential, but randomized to spread reconnection attempts:
   ```javascript
   const delay = Math.min(baseDelay * Math.pow(2, attempt), maxDelay);
   const jitter = delay * 0.5 * Math.random();
   return delay + jitter;
   ```
2. **Maximum retry limit** - Stop trying after N attempts; show user action required
3. **Connection state machine** - Track: disconnected, connecting, connected, reconnecting
4. **Graceful degradation** - If WebSocket unavailable, show clear status (not spinning forever)
5. **Server-side connection limiting** - Rate limit new connections per IP/time window
6. **Health check before reconnect** - Ping server health endpoint before attempting WebSocket

**Detection (warning signs):**
- CPU spikes during deployments
- Monitoring shows connection count spike, then crash
- Users report issues clustered around deployment times

**Phase:** Phase 2 (WebSocket infrastructure) - Must be designed correctly from the start

**Confidence:** HIGH - Well-documented pattern in WebSocket literature

**Sources:**
- [WebSocket Reconnection Logic (OneUptime)](https://oneuptime.com/blog/post/2026-01-24-websocket-reconnection-logic/view)
- [Robust WebSocket Reconnection with Exponential Backoff (DEV)](https://dev.to/hexshift/robust-websocket-reconnection-strategies-in-javascript-with-exponential-backoff-40n1)
- [Deal with Reconnection Storm (Medium)](https://amirsoleimani.medium.com/deal-with-reconnection-storm-two-strategies-4a835d0457f6)

---

### Pitfall 4: STT Streaming Latency Accumulation

**What goes wrong:** Latency compounds across the pipeline, turning a "real-time" system into one with multi-second delays that break the "magical" experience.

**Why it happens:** Multiple stages each add latency:
- Audio capture buffer (50-200ms)
- Network transmission (50-200ms depending on connection)
- Whisper chunk processing (100-500ms per chunk)
- VAD/endpointing delays (200-500ms waiting for speech end)
- LLM inference (200-2000ms)
- Content decryption and serving (variable)

**Vendor claims are misleading:** "Sub-300ms latency" often excludes network latency, chunk buffering, and VAD delays. Real-world latency can easily reach 2-5 seconds.

**Consequences:**
- Child says "show me a cat" and waits 3+ seconds - feels broken
- Toddlers lose attention and repeat/change requests mid-processing
- System responds to old requests while new ones queue
- The "magical instant" experience is impossible

**Prevention:**
1. **Measure end-to-end, not component** - Instrument from "audio captured" to "content displayed"
2. **Small audio chunks** - 100-200ms chunks for faster partial results
3. **Streaming partial transcripts** - Show transcription as it arrives (visual feedback)
4. **Aggressive endpointing** - Don't wait for long silences; for "show me a cat" you know intent quickly
5. **LLM streaming** - Start processing as transcription arrives, not after complete
6. **Preload content** - If concept database is small, preload thumbnails/metadata
7. **Optimistic UI** - Show "thinking" states immediately; never leave UI frozen
8. **Consider faster models** - Whisper large is accurate but slow; try distil-whisper or Whisper small for lower latency

**Detection (warning signs):**
- User testing shows "it feels slow"
- Logging shows >2s from audio start to response
- Children repeat commands before response appears

**Phase:** Spans Phase 1 (audio) through Phase 3 (LLM integration) - Requires holistic optimization

**Confidence:** HIGH - Multiple benchmarks and production reports confirm latency challenges

**Sources:**
- [Understanding and Reducing Latency in Speech-to-Text APIs (Deepgram)](https://deepgram.com/learn/understanding-and-reducing-latency-in-speech-to-text-apis)
- [How to Measure Latency in Speech-to-Text (Gladia)](https://www.gladia.io/blog/measuring-latency-in-stt)
- [Speech-to-Text Latency (Picovoice)](https://picovoice.ai/blog/speech-to-text-latency/)

---

## Moderate Pitfalls

Mistakes that cause delays or technical debt.

---

### Pitfall 5: LLM Intent Detection on Noisy Transcription

**What goes wrong:** The LLM receives garbled transcription and either hallucinates intent, fails to recognize valid requests, or gets confused by conversational fragments.

**Why it happens:**
- STT errors compound with children's speech patterns
- Continuous listening captures background speech (parents talking, TV)
- Transcription may include fragments: "show me a... no wait... cat!"
- LLM prompt not designed for noise tolerance

**Consequences:**
- False positives: Responds to background conversation
- False negatives: Misses valid commands
- Inappropriate responses: Interprets partial speech as complete command
- Prompt injection risk: Transcription contains unexpected text that manipulates LLM

**Prevention:**
1. **Design prompts for noise** - Explicitly instruct LLM that input is noisy transcription from a child
2. **Provide the vocabulary** - Pass the concept list to the LLM; it should match to known concepts
3. **Confidence thresholds** - LLM should output confidence; only act on high-confidence detections
4. **Require minimal viable phrase** - "Show X" or "X please" patterns, not just "cat" alone
5. **Context window management** - Don't accumulate too much historical transcription; recent context only
6. **Explicit "I heard X but I'm not sure" responses** - Avoid silent failures
7. **Sanitize before LLM** - Remove obviously non-speech content if detectable

**Detection (warning signs):**
- LLM responds to TV/radio background
- False triggers during parent conversations
- Inconsistent responses to same phrase

**Phase:** Phase 3 (LLM integration)

**Confidence:** MEDIUM - Patterns documented but toddler-specific prompt engineering is less documented

**Sources:**
- [Intent Detection using LLM (DSWithMac)](https://dswithmac.com/posts/intent-detection/)
- [5 Tips to Optimize LLM Intent Classification Prompts (Voiceflow)](https://www.voiceflow.com/pathways/5-tips-to-optimize-your-llm-intent-classification-prompts)
- [Prompt Engineering Guide 2026 (Lakera)](https://www.lakera.ai/blog/prompt-engineering-guide)

---

### Pitfall 6: Continuous Listening Battery and Resource Drain

**What goes wrong:** Always-on microphone capture drains mobile device batteries and consumes excessive CPU, making the app impractical for extended use.

**Why it happens:**
- Microphone capture requires continuous audio processing
- Streaming to server uses network continuously
- No intelligent gating of when to process vs. ignore
- Mobile browsers don't have access to low-power DSPs

**Consequences:**
- Parents complain app "kills my battery"
- Device heats up during use
- Unpractical for the "leave it running while kid plays" use case
- Potentially excessive data usage on mobile

**Prevention:**
1. **Client-side VAD first** - Only stream audio when speech detected
2. **Silero VAD in browser** - Use WASM-compiled VAD for efficient speech detection
3. **Visual microphone toggle** - Make it easy to pause listening
4. **Auto-pause on inactivity** - Stop processing after N minutes of no valid commands
5. **Optimize audio quality settings** - Lower sample rates (16kHz sufficient for speech) reduce processing
6. **Background tab handling** - Pause/reduce activity when tab not visible

**Detection (warning signs):**
- High CPU usage during idle
- Network transfer rate constant even during silence
- Battery usage complaints in testing

**Phase:** Phase 1-2 (audio pipeline optimization)

**Confidence:** MEDIUM - Browser-based continuous listening is less documented than native apps

**Sources:**
- [Design Considerations for Low-Power Always-On Voice Systems (Embedded)](https://www.embedded.com/design-considerations-for-low-power-always-on-voice-command-systems/)
- [Continuous Speech Data in Smart Devices (Audio-to-Text)](https://audio-to-text.com/resource/smart-devices-use-continuous-speech-data/)

---

### Pitfall 7: Age Encryption Performance on Large Media Files

**What goes wrong:** Decrypting video files on-demand introduces visible delay, especially for larger files or when serving multiple requests.

**Why it happens:**
- Age encryption is secure but not optimized for streaming
- Default implementation decrypts entire file to temp file before serving
- Large videos (100MB+) take seconds to decrypt
- Temp file cleanup can fail, leaving unencrypted files on disk

**Consequences:**
- Delay between voice command and content display
- Disk space bloat from orphaned temp files
- Potential security issue if temp files persist

**Prevention:**
1. **Streaming decryption** - Use `DecryptReader` to stream, not `DecryptToFile`
2. **Cache decrypted content in memory** - For frequently accessed content (concept database is finite)
3. **Optimize content file sizes** - Compress videos appropriately; 720p usually sufficient for kids content
4. **Preload likely content** - If LLM suggests "cat" might be coming, start decryption early
5. **Robust temp file cleanup** - Use `defer` with error handling; consider tmpfs for temp files
6. **Consider content CDN layer** - Decrypt once, cache in CDN with short TTL

**Detection (warning signs):**
- Noticeable delay between command and content display
- Disk usage growing over time
- Temp directory contains old `.mp4` files

**Phase:** Phase 4 (content serving)

**Confidence:** HIGH - Based on subtitler project patterns and age library documentation

**Sources:**
- [age package documentation (Go Packages)](https://pkg.go.dev/filippo.io/age)
- [AES-256 File Encryption in Go (Medium)](https://omept-tech.medium.com/aes-256-file-encryption-in-go-encrypting-large-video-files-without-modifying-the-source-b24f0e0336bc)
- Subtitler project encryption spec (internal reference)

---

### Pitfall 8: React Audio State Desynchronization

**What goes wrong:** Audio playback state (playing, paused, position) gets out of sync with React component state, causing UI to show incorrect status or audio to behave unexpectedly.

**Why it happens:**
- Audio elements have their own internal state separate from React
- Frequent updates (streaming) can cause race conditions with setState
- Functional state updates required but often forgotten
- External events (audio ended, error) not properly handled

**Consequences:**
- Play button shows "playing" when audio is stopped
- Multiple audio elements playing simultaneously
- Memory leaks from orphaned audio contexts
- UI freezes or becomes unresponsive

**Prevention:**
1. **Use functional setState** - `setState(prev => ...)` not `setState(newValue)`
2. **Single source of truth** - Audio element is truth; React state mirrors it
3. **Event-driven sync** - Update React state from audio events (onPlay, onPause, onEnded)
4. **Cleanup on unmount** - Stop audio and release resources in useEffect cleanup
5. **Consider audio state library** - Libraries like `react-use-audio-player` handle edge cases
6. **Web Audio API for complex cases** - More control but more complexity

**Detection (warning signs):**
- UI state doesn't match what's playing
- Multiple sounds playing at once
- Console errors about state updates on unmounted components

**Phase:** Phase 4 (content display/playback)

**Confidence:** MEDIUM - Well-documented React pattern but audio-specific issues less covered

**Sources:**
- [Handling State Update Race Conditions in React (Medium)](https://medium.com/cyberark-engineering/handling-state-update-race-conditions-in-react-8e6c95b74c17)
- [Building an Audio Player in React (LogRocket)](https://blog.logrocket.com/building-audio-player-react/)

---

### Pitfall 9: VAD False Positives in Home Environment

**What goes wrong:** Voice Activity Detection triggers on non-speech sounds (TV, pets, household noise), causing spurious processing and potential false responses.

**Why it happens:**
- VAD is designed for clean audio, not living rooms
- WebRTC VAD has 50% true positive rate at 5% false positive rate
- Children's environments are noisy (toys, siblings, TV)
- Keyboard clicks, coughs, background music trigger VAD

**Consequences:**
- STT processes noise, producing garbage transcription
- LLM receives noise-transcription, may hallucinate intent
- Battery/CPU waste on non-speech processing
- Erratic behavior that confuses users

**Prevention:**
1. **Use modern VAD** - Silero VAD (87.7% TPR) or Cobra (98.9% TPR) significantly outperform WebRTC
2. **Multi-stage detection** - VAD for first pass, then verify with STT confidence
3. **Noise classifier** - Detect common non-speech sounds (music, TV) and filter
4. **Confidence threshold** - Only process high-confidence VAD detections
5. **Acoustic event detection** - Identify and ignore known non-speech events
6. **User-configurable sensitivity** - Let parents adjust for their environment

**Detection (warning signs):**
- High VAD trigger rate during silence
- STT frequently returns "[NOISE]" or garbled text
- Processing happens when no one is speaking

**Phase:** Phase 1-2 (audio pipeline)

**Confidence:** HIGH - Benchmarks clearly document VAD performance differences

**Sources:**
- [Best Voice Activity Detection 2026: Cobra vs Silero vs WebRTC (Picovoice)](https://picovoice.ai/blog/best-voice-activity-detection-vad/)
- [Voice Activity Detection Guide 2025 (Picovoice)](https://picovoice.ai/blog/complete-guide-voice-activity-detection-vad/)

---

## Minor Pitfalls

Mistakes that cause annoyance but are fixable.

---

### Pitfall 10: Whisper Streaming Buffer Configuration

**What goes wrong:** Incorrect buffer/chunk size configuration causes either poor latency (too large) or poor accuracy (too small).

**Why it happens:**
- Whisper designed for batch processing, not streaming
- Streaming implementations require tuning MinChunkSize
- Default configurations optimized for accuracy, not latency
- Buffer trimming affects both quality and responsiveness

**Consequences:**
- Large chunks (1+ seconds): Good accuracy but noticeable delay
- Small chunks (<100ms): Fast response but fragmented transcription
- Wrong trimming: Cuts words mid-utterance

**Prevention:**
1. **Start with recommended defaults** - MinChunkSize around 0.5-1.0 seconds
2. **Tune for use case** - Short commands ("show me cat") tolerate smaller chunks
3. **Use local agreement policy** - Whisper-streaming's approach for stable output
4. **Monitor both metrics** - Track latency AND accuracy together
5. **Consider faster models** - distil-whisper or whisper-small trade accuracy for speed

**Detection (warning signs):**
- Words cut off mid-transcription
- Long delay before first word appears
- Transcription quality varies wildly

**Phase:** Phase 1 (Whisper integration)

**Confidence:** HIGH - Whisper-streaming documentation covers this well

**Sources:**
- [Whisper Streaming (GitHub)](https://github.com/ufal/whisper_streaming)
- [Turning Whisper into Real-Time Transcription System (arXiv)](https://arxiv.org/html/2307.14743)

---

### Pitfall 11: Missing Visual Feedback During Voice Processing

**What goes wrong:** Users (especially children) don't know if the system heard them, is processing, or has failed.

**Why it happens:**
- Focus on backend functionality over UX
- Multiple async stages make state management complex
- "It works in demo" mentality ignores real-world confusion
- Children can't read error messages

**Consequences:**
- Children repeat commands (compounding processing)
- Parents think app is broken
- No way to distinguish "processing" from "failed"
- Frustrating experience even when system works

**Prevention:**
1. **Visual listening indicator** - Clear animation when mic is active (the red indicator in requirements)
2. **Live transcription display** - Show words as they're recognized
3. **Processing state** - Visual indicator when LLM is thinking
4. **Success/failure feedback** - Clear indication when content loads or command fails
5. **Child-friendly visuals** - No text-heavy error messages; use animations
6. **Audio feedback option** - Sound when listening starts/stops

**Detection (warning signs):**
- User testing shows confusion about app state
- Children repeatedly say "hello?" or "are you there?"
- Parents explain the app to children

**Phase:** Phase 2-3 (frontend UX)

**Confidence:** HIGH - Well-documented UX pattern

**Sources:**
- [Designing Voice Assistants for Children (UX Collective)](https://uxdesign.cc/designing-voice-assistants-for-children-b6861870359)
- [UX for Kids: Designing Experiences for Toddlers 2026 (BitsKingdom)](https://bitskingdom.com/blog/ux-for-kids-gen-alpha-toddlers/)

---

### Pitfall 12: Toddler-Inappropriate Interaction Patterns

**What goes wrong:** Interface requires gestures or interactions that toddlers cannot perform (swipes, double-taps, reading).

**Why it happens:**
- Default to adult interaction patterns
- Copy existing voice assistant UX designed for adults
- Underestimate toddler limitations
- Not testing with actual toddlers

**Consequences:**
- Toddlers can't operate app independently
- Frustration for both children and parents
- App becomes parent-operated tool, losing "magical" quality

**Prevention:**
1. **Single tap only** - No double-tap, long-press, or swipes for primary actions
2. **Large touch targets** - Minimum 48x48px, ideally larger for toddlers
3. **Immediate feedback** - Visual/audio confirmation on every tap
4. **Voice-first design** - Core interaction is voice; touch is supplementary
5. **Parent gates for settings** - Hidden or protected access to configuration
6. **No reading required** - Icons and images only for child-facing UI

**Detection (warning signs):**
- Toddlers struggle with UI elements
- Parents have to help with every interaction
- High accidental trigger rate for advanced features

**Phase:** Phase 2-4 (frontend design)

**Confidence:** HIGH - Child UX research is well-documented

**Sources:**
- [UX Design for Kids: Principles and Recommendations (Ramotion)](https://www.ramotion.com/blog/ux-design-for-kids/)
- [Top 10 UI/UX Design Tips for Child-Friendly Interfaces (AufaitUX)](https://www.aufaitux.com/blog/ui-ux-designing-for-children/)

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Audio capture setup | Mobile browser permissions | Test on actual iOS/Android devices from day 1 |
| Whisper integration | Streaming latency accumulation | Benchmark end-to-end early, not just component |
| WebSocket infrastructure | Reconnection storms | Implement exponential backoff + jitter upfront |
| LLM intent detection | Child speech errors | Design prompts for noise tolerance with concept vocabulary |
| Content serving | Age decryption latency | Use streaming decryption, consider in-memory cache |
| Frontend UX | State desynchronization | Functional setState, audio-event-driven updates |
| Testing phase | Adult speech bias | Capture and test with real toddler audio samples |

---

## Cross-Cutting Concerns

### The Latency Budget

For a "magical" experience, target <2 seconds from voice to content. Budget allocation:

| Stage | Target | Notes |
|-------|--------|-------|
| Audio capture + VAD | 200ms | Client-side, includes chunk buffer |
| Network to Whisper | 100ms | Depends on deployment topology |
| Whisper processing | 500ms | Use fast model, small chunks |
| LLM inference | 500ms | Stream response, act on partial |
| Content decryption | 200ms | Cache common content |
| Content display | 100ms | Preload assets |
| **Total** | **1600ms** | Leaves 400ms margin |

Exceeding 2s requires UX mitigation (progress indicators, partial responses).

### The Accuracy Budget

For usable experience with toddlers, target >70% command success rate:

| Factor | Impact | Mitigation |
|--------|--------|------------|
| Child ASR accuracy | High | Phonetic matching, vocabulary priors |
| VAD false positives | Medium | Modern VAD (Silero/Cobra) |
| LLM intent errors | Medium | Noise-tolerant prompts, confidence thresholds |
| Network failures | Low | Graceful degradation, retry logic |

---

## Sources Summary

### Official Documentation (HIGH confidence)
- [MDN getUserMedia()](https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getUserMedia)
- [age package (Go Packages)](https://pkg.go.dev/filippo.io/age)
- [Whisper Streaming (GitHub)](https://github.com/ufal/whisper_streaming)

### Academic/Research (HIGH confidence)
- [Child Speech Recognition PMC Papers](https://pmc.ncbi.nlm.nih.gov/articles/PMC7199459/)
- [Whisper Real-Time Transcription (arXiv)](https://arxiv.org/html/2307.14743)

### Industry Benchmarks (MEDIUM confidence)
- [VAD Comparison 2026 (Picovoice)](https://picovoice.ai/blog/best-voice-activity-detection-vad/)
- [STT Latency Guide (Picovoice)](https://picovoice.ai/blog/speech-to-text-latency/)
- [STT Latency (Deepgram)](https://deepgram.com/learn/understanding-and-reducing-latency-in-speech-to-text-apis)

### Developer Experience (MEDIUM confidence)
- [WebSocket Reconnection Patterns](https://oneuptime.com/blog/post/2026-01-24-websocket-reconnection-logic/view)
- [Common getUserMedia Errors](https://blog.addpipe.com/common-getusermedia-errors/)

### Child UX Design (MEDIUM confidence)
- [Designing Voice Assistants for Children](https://uxdesign.cc/designing-voice-assistants-for-children-b6861870359)
- [UX for Kids 2026](https://bitskingdom.com/blog/ux-for-kids-gen-alpha-toddlers/)
