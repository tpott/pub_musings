# Speech-to-Text Provider Research

This document compares alternative speech-to-text (STT) providers to Whisper for use in the subtitler project. The goal is to evaluate providers based on features needed for high-quality subtitle generation.

## Current Implementation

The subtitler currently uses **OpenAI Whisper** (large-v3 or large-v3-turbo) via whisper-server running on the host machine. This provides:
- Self-hosted (no API costs)
- Word-level timestamps
- Multi-language support (99+ languages)
- No data sent to third parties

## Evaluation Criteria

For subtitle generation, we prioritize:
1. **Word-level timestamps** - Essential for syncing subtitles to video
2. **Sentence alignment** - Natural grouping of words into readable segments
3. **Speaker diarization** - Identifying different speakers
4. **Accuracy (WER)** - Word Error Rate on diverse audio
5. **Phoneme support** - For language learning features
6. **Speaker tone/sentiment detection** - For advanced subtitle styling
7. **Pricing** - Cost per minute of audio

---

## Provider Comparison

### 1. AssemblyAI

**Overview:** Cloud-based STT API with extensive audio intelligence features.

**Features:**
| Feature | Support | Notes |
|---------|---------|-------|
| Word-level timestamps | ✅ Yes | Millisecond precision |
| Sentence alignment | ✅ Yes | Automatic punctuation and capitalization |
| Speaker diarization | ✅ Yes | 16 languages, up to 10 speakers |
| Languages | 99 | Automatic language detection |
| Phoneme support | ❌ No | Not available |
| Sentiment analysis | ✅ Yes | Per-sentence positive/negative/neutral |
| Emotion detection | ✅ Yes | Happy, angry, satisfied, etc. |

**Accuracy:**
- Universal-2 model achieves 8.4% WER (best in class)
- 30% fewer hallucinations than Whisper Large-v3
- Strong performance with noisy audio

**Pricing (2026):**
- Pre-recorded: $0.27/hour ($0.0045/min)
- Streaming: $0.15/hour ($0.0025/min)
- Speaker diarization: +$0.02/hour
- Sentiment analysis: +$0.02/hour
- Free tier: $50 credit (~185 hours)

**Pros:**
- Best accuracy among cloud providers
- Comprehensive audio intelligence features
- Simple per-second billing
- Good documentation

**Cons:**
- Cloud-only (data leaves your infrastructure)
- Add-on costs for features
- No phoneme-level timestamps

---

### 2. Deepgram

**Overview:** Fast, accurate cloud STT with emphasis on real-time streaming.

**Features:**
| Feature | Support | Notes |
|---------|---------|-------|
| Word-level timestamps | ✅ Yes | High precision with Nova-3 |
| Sentence alignment | ✅ Yes | Utterances feature |
| Speaker diarization | ✅ Yes | Built-in, no extra charge |
| Languages | 36 | Major languages covered |
| Phoneme support | ❌ No | Not available |
| Sentiment analysis | ✅ Yes | Via add-on |
| Smart formatting | ✅ Yes | Dates, numbers, addresses |

**Accuracy:**
- Nova-3 model: ~18% WER on mixed datasets
- 36% lower WER than Whisper on select datasets
- Fastest transcription (~300ms latency)

**Pricing (2026):**
- Pay-as-you-go: $0.0077/min ($0.46/hour)
- Growth plan: $0.0065/min (annual commitment)
- High volume: As low as $0.003/min
- Free tier: $200 credit (~26,000 minutes)

**Pros:**
- Fastest real-time transcription
- Built-in diarization at no extra cost
- Generous free tier
- Good developer experience

**Cons:**
- Fewer languages than competitors
- Less audio intelligence than AssemblyAI
- Cloud-only

---

### 3. Google Cloud Speech-to-Text

**Overview:** Enterprise-grade STT with extensive language support.

**Features:**
| Feature | Support | Notes |
|---------|---------|-------|
| Word-level timestamps | ✅ Yes | 100ms increments |
| Sentence alignment | ✅ Yes | With punctuation |
| Speaker diarization | ✅ Yes | Chirp 3 model |
| Languages | 125+ | Best language coverage |
| Phoneme support | ❌ No | Not in standard API |
| Speech adaptation | ✅ Yes | Custom vocabulary boosting |
| Built-in denoiser | ✅ Yes | Chirp 3 only |

**Accuracy:**
- Chirp 3: Best batch transcription accuracy
- 11.6% WER in some benchmarks
- Struggles with challenging audio conditions

**Pricing (2026):**
- Standard: $0.016/min ($0.96/hour)
- Batch: $0.004/min ($0.24/hour)
- Free tier: 60 minutes/month
- Additional: Storage, functions, egress fees

**Pros:**
- Best language coverage (125+)
- Enterprise features (audit logging, CMEK)
- Integrates well with GCP ecosystem

**Cons:**
- Lower accuracy than specialized providers
- Complex pricing with hidden costs
- Requires GCP account setup

---

### 4. AWS Transcribe

**Overview:** Amazon's STT service, integrated with AWS ecosystem.

**Features:**
| Feature | Support | Notes |
|---------|---------|-------|
| Word-level timestamps | ✅ Yes | Start/end times per word |
| Sentence alignment | ✅ Yes | Automatic punctuation |
| Speaker diarization | ✅ Yes | Up to 5 speakers recommended |
| Languages | 100+ | Good coverage |
| Phoneme support | ❌ No | Not available |
| Medical transcription | ✅ Yes | Separate product |
| PII redaction | ✅ Yes | Built-in |

**Accuracy:**
- Solid real-time performance
- Struggles with background noise and overlapping speakers
- Works reliably for clear recordings

**Pricing (2026):**
- Tier 1 (0-250K min): $0.024/min ($1.44/hour)
- Tier 2 (250K-1M min): $0.015/min
- Tier 3 (1M-5M min): $0.0102/min
- Tier 4 (5M+ min): $0.0078/min
- Medical: $0.075/min
- Free tier: 60 min/month for 12 months
- Additional charges for diarization, PII redaction

**Pros:**
- Deep AWS integration
- Volume discounts
- Built-in PII redaction
- Medical transcription option

**Cons:**
- More expensive at low volume
- Accuracy not best in class
- Additional charges for features

---

### 5. Specialized Providers

#### Speechace (Pronunciation Scoring)
For language learning features that need phoneme-level analysis:

**Features:**
- Phoneme-level timestamps and scoring
- Syllable-level analysis
- Pronunciation quality assessment (0-100 scale)
- IELTS/PTE score estimation
- Languages: 8 (English, Chinese, Korean, Japanese, German, French, Spanish, Russian)

**Use case:** If we add pronunciation assessment for language learners, Speechace would complement our STT provider.

#### SpeechSuper (Similar to Speechace)
- Phoneme, syllable, word, and sentence level assessment
- Fluency metrics (words correct per minute, pause count)
- API for custom vocabulary

---

## Feature Matrix Summary

| Feature | Whisper | AssemblyAI | Deepgram | Google | AWS |
|---------|---------|------------|----------|--------|-----|
| Word timestamps | ✅ | ✅ | ✅ | ✅ | ✅ |
| Diarization | ❌ | ✅ | ✅ | ✅ | ✅ |
| Phoneme support | ❌ | ❌ | ❌ | ❌ | ❌ |
| Sentiment | ❌ | ✅ | ✅ | ❌ | ❌ |
| Languages | 99 | 99 | 36 | 125+ | 100+ |
| Self-hosted | ✅ | ❌ | ❌ | ❌ | ❌ |
| Accuracy (WER) | 7-9% | 8% | 18% | 11% | varies |

## Pricing Comparison (per minute)

| Provider | Base Rate | With Diarization | Notes |
|----------|-----------|------------------|-------|
| Whisper | $0.00 | $0.00 | Self-hosted, GPU costs |
| AssemblyAI | $0.0045 | $0.0048 | Best accuracy |
| Deepgram | $0.0077 | $0.0077 | Diarization included |
| Google | $0.016 | $0.016+ | Enterprise features |
| AWS | $0.024 | $0.024+ | Volume discounts |

---

## Recommendations

### Keep Whisper (Current)
**Best for:** Privacy-focused users, cost-conscious deployment, self-hosted requirements.

Whisper remains the best choice for our current use case:
- No per-minute API costs
- Data never leaves user's infrastructure
- Excellent accuracy (7-9% WER)
- 99+ language support
- Full control over model and processing

### Consider AssemblyAI Hybrid
**Best for:** Users willing to pay for better accuracy and audio intelligence.

If we want to offer a premium tier:
- Use AssemblyAI for sentiment analysis and diarization
- Fall back to Whisper when AssemblyAI is unavailable
- Estimated cost: ~$2.70/hour for full features

### Consider Deepgram for Real-time
**Best for:** Live captioning or streaming use cases.

If we add live transcription:
- Best latency (<300ms)
- Built-in diarization
- Generous free tier for development

### Skip Google/AWS
**Rationale:**
- Higher costs than specialized providers
- Lower accuracy than AssemblyAI/Deepgram
- More complex integration
- Only consider if already heavily invested in those ecosystems

---

## Future Considerations

### Phoneme-Level Timestamps
None of the mainstream STT providers offer phoneme-level timestamps. For language learning features requiring pronunciation assessment:
1. Use Speechace or SpeechSuper as a secondary API
2. Investigate forced alignment tools (Montreal Forced Aligner, Gentle)
3. Consider fine-tuning Whisper with phoneme data

### Speaker Tone Detection
AssemblyAI's sentiment analysis is the most mature option. Integration would require:
1. Sending audio to AssemblyAI (privacy consideration)
2. Storing sentiment per segment
3. UI for displaying tone/mood indicators

### Diarization Without Cloud
For self-hosted diarization:
1. pyannote.audio - Open-source, GPU-accelerated
2. NeMo Speaker Diarization - NVIDIA's toolkit
3. Could be integrated with Whisper pipeline

---

## Implementation Notes

If switching or adding providers:

1. **Abstract the STT interface** - Create a common interface that wraps provider-specific APIs
2. **Handle fallback** - If cloud provider fails, fall back to Whisper
3. **Cache results** - Store transcriptions to avoid re-processing
4. **Track costs** - Log usage for billing purposes if using paid APIs
5. **Privacy controls** - Let users opt-in to cloud transcription

Example interface:
```go
type STTProvider interface {
    Transcribe(audioPath string, opts TranscribeOptions) (*Transcription, error)
    GetCapabilities() Capabilities
}

type Capabilities struct {
    WordTimestamps     bool
    Diarization        bool
    SentimentAnalysis  bool
    Languages          []string
    MaxDurationMinutes int
}
```

---

## Conclusion

**Whisper remains the best default** for our subtitle generator due to:
- Zero marginal cost per transcription
- Privacy (no data sent to third parties)
- Competitive accuracy
- Self-hosted control

**AssemblyAI is the best cloud alternative** if we need:
- Speaker diarization
- Sentiment analysis
- Premium accuracy tier

**Deepgram is best for real-time** if we add live captioning features.

**For phoneme-level features**, we'd need to integrate a specialized provider like Speechace.

---

## Sources

- [AssemblyAI Pricing](https://www.assemblyai.com/pricing)
- [AssemblyAI Benchmarks](https://www.assemblyai.com/benchmarks)
- [AssemblyAI Speaker Diarization](https://www.assemblyai.com/speaker-diarization)
- [Deepgram Pricing](https://deepgram.com/pricing)
- [Deepgram Speech-to-Text Benchmarks](https://deepgram.com/learn/speech-to-text-benchmarks)
- [Deepgram Diarization Docs](https://developers.deepgram.com/docs/diarization)
- [Google Cloud Speech-to-Text Pricing](https://cloud.google.com/speech-to-text/pricing)
- [Google Chirp 3 Documentation](https://docs.cloud.google.com/speech-to-text/docs/models/chirp-3)
- [AWS Transcribe Pricing](https://aws.amazon.com/transcribe/pricing/)
- [AWS Transcribe Diarization](https://docs.aws.amazon.com/transcribe/latest/dg/diarization.html)
- [Speechace API Documentation](https://docs.speechace.com/)
- [SpeechSuper API](https://www.speechsuper.com/)
- [Best Open Source STT Models 2026](https://northflank.com/blog/best-open-source-speech-to-text-stt-model-in-2026-benchmarks)
- [STT Provider Comparison](https://artificialanalysis.ai/speech-to-text)
