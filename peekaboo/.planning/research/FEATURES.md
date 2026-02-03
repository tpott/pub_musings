# Feature Landscape: Voice-Activated Kids Content Viewer

**Domain:** Voice-activated educational/entertainment content viewer for toddlers
**Researched:** 2026-02-02
**Confidence:** MEDIUM (synthesized from multiple web sources, industry patterns, and competitive analysis)

## Table Stakes

Features users (parents and toddlers) expect. Missing any of these makes the product feel incomplete or unsafe.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Visible Listening Indicator** | Parents and kids need to know when the mic is active. 93% of parents say knowing when voice is recorded is important. | Low | Red pulsing indicator, animated waveform, or mic icon. Must be obvious even to toddlers. |
| **Mic Toggle Button** | Parents must be able to turn off listening. Standard in all voice devices (Alexa, Google). Required for privacy control. | Low | Large, obvious, physically accessible. Visual state change (red=on, gray=off). |
| **Immediate Visual Feedback** | "The voice-to-visual loop must feel instant and magical." Kids need immediate response to maintain engagement. | Medium | Show content within 1-2 seconds of intent detection. Loading states frustrate toddlers. |
| **Audio Feedback for Actions** | Sound is one type of feedback kids appreciate and pay the most attention to. Confirms the app "heard" them. | Low | Chime when listening starts, confirmation sound when request understood, content-appropriate audio with results. |
| **Large Touch Targets** | Toddlers have developing motor skills. Standard kids UX requires 48x48px minimum, preferably larger. | Low | All interactive elements must be large and tappable. |
| **Curated, Pre-Approved Content Only** | Parents expect complete content safety. "Curated content with no user-generated material" is the gold standard. | Low | No external API content in v1. Hand-picked photos/videos only. |
| **No Ads or In-App Purchases** | Expected for toddler apps. Google's Read Along and similar successful kids apps are ad-free. | Low | Zero monetization in the kid-facing experience. |
| **Bright, High-Contrast Visuals** | Toddler UX best practices: "bright colors, simple shapes, and captivating sounds." | Low | Content display area should be colorful with clear focus. |
| **Simple Error Recovery** | Voice recognition fails 50% of the time for children. App must gracefully handle "not understood" states. | Medium | Gentle "try again" prompt with visual cue, not frustrating error messages. |
| **Content Display Full-Screen** | Maximizes visual impact, reduces distractions. Standard for media viewers. | Low | Photo/video fills display area when showing content. |

## Differentiators

Features that set Peekaboo apart. Not expected in basic apps, but create competitive advantage.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **No Wake Word Required** | More natural interaction. LLM judges intent from context. "Show me a cat" just works without "Hey Peekaboo" first. | High | Core differentiator. Requires sophisticated intent detection. V1 explicit goal. |
| **Continuous Streaming STT** | Low-latency response. Words appear as spoken, creating immediate feedback loop. | High | Whisper streaming architecture. Key to "instant and magical" feel. |
| **Real-time Transcription Display** | Shows speech is being understood even before content loads. Builds confidence for parents and kids. | Medium | Scrolling dialog box per PROJECT.md. Visible feedback during processing. |
| **Multi-Modal Content Response** | Photo + audio (e.g., cat photo + meow sound). Richer than single-media response. | Medium | Optional audio clips tied to concepts. More engaging than photo alone. |
| **Context-Aware Intent Detection** | Handles parent speaking on behalf of child ("show her a dog"). Understands conversation context. | High | LLM-based, not pattern matching. Supports natural family interactions. |
| **Offline-First Architecture** | Works without internet for content display. All content stored locally encrypted. | Medium | Valuable for airplane mode, road trips, spotty connectivity. Privacy benefit: no cloud dependency for media. |
| **Dual LLM Provider Support** | Flexibility to switch between Anthropic/OpenAI based on quality/cost. Future-proof. | Medium | Abstracted provider interface. Competitive advantage in reliability. |
| **Admin Content Curation Pipeline** | Python REPL + helper scripts for review. Separates admin from kid experience. | Medium | Professional content management without compromising kid safety. |

## Anti-Features

Features to explicitly NOT build. Common mistakes in kids voice apps.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Text-to-Speech Responses** | Deferred to v2 per PROJECT.md. Adds complexity without core value. Kids want to SEE things, not hear AI talk. | Show content immediately. Audio clips (meow, bark) tied to content are different from TTS narration. |
| **Rigid Wake Word Detection** | Unnatural for kids. Creates frustration. "Hey Peekaboo" before every request breaks flow. | LLM intent detection from context. "Show me a cat" should just work. |
| **User-Generated Content** | Safety nightmare. Moderation impossible. COPPA compliance risk. | 100% curated, hand-picked content only. |
| **Cloud-Stored Voice Recordings** | Privacy risk. FTC fined Amazon $25M for retaining children's voice data. | Process voice on-device or via streaming STT. Don't persist recordings. |
| **Complex Navigation** | Toddlers don't follow linear navigation. Multi-step flows fail. | Single-screen app. Voice command = content appears. No menus for kids. |
| **Weather/Time/Calculator Tools** | Out of scope per PROJECT.md. Distracts from core "show me a thing" value. | v2 feature. Keep v1 focused on content display. |
| **API-Sourced Content** | External APIs (Unsplash, Pexels) introduce uncontrolled content, latency, failure modes. | v2 enhancement. v1 uses only local encrypted content. |
| **Gamification / Points / Badges** | Adds complexity. Not needed for core value of "see a thing." | Keep it simple. Content is the reward. |
| **Long Session Design** | Toddler attention spans are 3-8 minutes. Designing for long engagement is developmentally inappropriate. | Expect short sessions. Quick in, quick out. |
| **Continuous Conversation Mode** | Privacy risk. Hard to implement well. Alexa's version is controversial. | Each request is independent. Mic toggle gives explicit control. |
| **Child Voice Training** | Requires storing voice samples. COPPA risk. Complexity without proportional benefit. | Accept that child speech recognition has 37% higher error rates. Graceful error handling instead. |
| **Profile-Based Personalization** | Requires data collection on children. COPPA compliance burden. | Single anonymous session. No accounts, no tracking. |

## Feature Dependencies

```
Core Dependencies:
STT Streaming ─────────────────┐
                               v
                    Intent Detection (LLM)
                               │
                               v
                    Content Lookup (SQLite)
                               │
                               v
                    Content Display (React)
                               │
                               v
                    Media Playback (Photo/Video/Audio)

Supporting Dependencies:
Age Encryption ─────> Content Storage ─────> Content Lookup
Admin API ─────────> Content Management ───> Content Storage

Visual Feedback Loop:
Mic Toggle ────> Listening Indicator ────> Transcription Display
                        │
                        v
               Audio Confirmation Chime
```

### Build Order Implications

1. **Content Display First** - Can be tested with mock data before STT works
2. **Content Database Second** - SQLite with encryption, populated manually
3. **STT Integration Third** - Requires whisper-stream service
4. **Intent Detection Fourth** - Requires working STT to test
5. **Admin Pipeline Last** - Can manually populate DB while building

## MVP Recommendation

For MVP, prioritize these in order:

### Must Have (Launch Blockers)
1. **Mic toggle with visual indicator** - Safety/privacy table stake
2. **Listening indicator** - Visible feedback that app is active
3. **Content display** - Photo/video full-screen with optional audio
4. **Content database** - SQLite with encrypted media files
5. **Streaming STT** - Whisper integration with real-time transcription
6. **LLM intent detection** - No wake word, context-aware
7. **Error handling** - Graceful "try again" for failed recognition

### Should Have (Quality of Life)
8. **Real-time transcription display** - Scrolling dialog box
9. **Audio feedback chimes** - Confirmation sounds
10. **Admin API** - Add content programmatically

### Defer to Post-MVP
- **TTS responses** - v2 per PROJECT.md
- **Weather/time tools** - v2 per PROJECT.md
- **API content sources** - v2, requires content moderation
- **Multiple content items per concept** - Start with 1:1 mapping
- **Video playback** - Start with photos + audio clips, add video later

## Session Design Recommendations

Based on toddler attention span research:

| Age | Expected Attention | Session Design |
|-----|-------------------|----------------|
| 2 years | 4-6 minutes | 3-5 content requests per session |
| 3 years | 6-8 minutes | 4-6 content requests per session |
| 4-5 years | 8-12 minutes | 6-10 content requests per session |

**Implication:** Design for quick, independent requests. No state between requests. Each "show me X" is self-contained.

## Safety and Privacy Requirements

### COPPA Compliance Essentials

| Requirement | How Peekaboo Addresses It |
|-------------|---------------------------|
| Parental consent for data collection | Mic toggle requires parent to enable. No persistent data storage. |
| Voice recordings are PII | Stream-process only, don't store. Transcriptions ephemeral. |
| No behavioral tracking | Anonymous sessions. No user profiles. |
| Content appropriate for children | 100% curated, hand-picked content. No external APIs. |
| Privacy policy | Need clear policy stating: no recording storage, no data collection on children. |

### Encryption Requirements

Per PROJECT.md context from subtitler:
- Age encryption for all media files (.age extension)
- Decrypt-on-demand for playback
- Key stored at `data/age.key` with 0600 permissions
- Never store decrypted content to disk

## Confidence Assessment

| Category | Confidence | Rationale |
|----------|------------|-----------|
| Table Stakes | HIGH | Consistent across multiple sources, industry standard patterns |
| Differentiators | MEDIUM | Based on competitive analysis, may need validation |
| Anti-Features | MEDIUM | Based on domain expertise and COPPA requirements |
| Attention Spans | HIGH | Multiple academic and developmental sources agree |
| COPPA Requirements | HIGH | FTC official guidance and enforcement actions |
| Build Order | MEDIUM | Logical dependencies but may need adjustment in practice |

## Sources

### Voice Interface Design
- [Futurice: 10 Principles for Designing Voice Services for Children](https://www.futurice.com/blog/how-to-design-great-voice-services-for-kids) - HIGH confidence
- [AuFait UX: Voice User Interface Design Best Practices](https://www.aufaitux.com/blog/voice-user-interface-design-best-practices) - MEDIUM confidence
- [Parallel HQ: VUI Design Principles](https://www.parallelhq.com/blog/voice-user-interface-vui-design-principles) - MEDIUM confidence

### Kids UX Patterns
- [Ramotion: UX Design for Kids](https://www.ramotion.com/blog/ux-design-for-kids/) - MEDIUM confidence
- [BitKingdom: UX for Kids - Designing for Toddlers](https://bitskingdom.com/blog/ux-for-kids-gen-alpha-toddlers/) - MEDIUM confidence
- [AuFait UX: UI/UX Design Tips for Child-Friendly Interfaces](https://www.aufaitux.com/blog/ui-ux-designing-for-children/) - MEDIUM confidence

### Speech Recognition for Children
- [TechCrunch: Voice Assistants Don't Work for Kids](https://techcrunch.com/2020/09/09/voice-assistants-dont-work-for-kids-the-problem-with-speech-recognition-in-the-classroom/) - HIGH confidence
- [ScienceDirect: Examining Voice Assistants in Context of Children's Speech](https://www.sciencedirect.com/science/article/abs/pii/S2212868922000587) - HIGH confidence
- [Sensory: Speech Recognition for Children](https://www.sensory.com/sensory-releases-speech-recognition-for-children/) - MEDIUM confidence

### Safety and Privacy
- [FTC: COPPA FAQ](https://www.ftc.gov/business-guidance/resources/complying-coppa-frequently-asked-questions) - HIGH confidence (official)
- [Promise Legal: COPPA Compliance 2025](https://blog.promise.legal/startup-central/coppa-compliance-in-2025-a-practical-guide-for-tech-edtech-and-kids-apps/) - MEDIUM confidence
- [Picovoice: Always Listening and Privacy](https://picovoice.ai/blog/voice-activation-hotword-detection/) - MEDIUM confidence

### Attention Spans
- [Brain Balance: Normal Attention Span Expectations by Age](https://www.brainbalancecenters.com/blog/normal-attention-span-expectations-by-age) - HIGH confidence
- [Happiest Baby: Attention Span of a Toddler](https://www.happiestbaby.com/blogs/toddler/attention-span) - HIGH confidence
- [Expressable: Guide to Understanding Your Toddler's Attention Span](https://www.expressable.com/learning-center/babies-and-toddlers/guide-to-understanding-your-toddlers-attention-span) - HIGH confidence

### Competitive Analysis
- [Amazon: Alexa for Kids Features](https://www.amazon.com/alexa-for-kids/b?ie=UTF8&node=21474972011) - HIGH confidence (official)
- [Amazon: Echo Show 5 Kids Setup](https://www.amazon.com/gp/help/customer/display.html?nodeId=TRGplEmEMVAEkQ52cE) - HIGH confidence (official)
- [Khan Academy: Offline Learning in Khan Academy Kids](https://khankids.zendesk.com/hc/en-us/articles/360029139531-Learn-on-the-go-with-offline-content-in-Khan-Academy-Kids) - HIGH confidence (official)

### Multimodal Design
- [Wings Design: Multi-Sensory UX](https://wings.design/multi-sensory-ux-integrating-haptics-sound-and-visual-cues-to-enhance-user-interaction/) - MEDIUM confidence
- [Educational Voice: Animation for Children's Content](https://educationalvoice.co.uk/animation-for-childrens-content/) - MEDIUM confidence
