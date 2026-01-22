# subtitler

A website for generating and editing subtitles for music videos and language learning content. Subtitles should be written in the script most native for reading that language—for example, Hindi subtitles should be in Devanagari (देवनागरी), not romanized transliteration.

## Core Function

When a user navigates to /upload or uses a drag-and-drop they should see real time
excerpts of their video overlayed instead of a spinner. As a user, I want to upload
my video, see the transcribed subtitles, and I want to be impressed by the accuracy
of the subtitles. I also want to have fun improving the alignment of subtitles, so
they start and end at the correct time or easily changing a single word.

If I upload video that I know the full transcript for, then I want to be able to paste
that in after the video processing has already started/finished. I want the server to
be smart and match the words it transcribed correctly and replace the words it missed.
I want the server to be smart about how it adjusts or aligns the subtitles with the
tokens that whisper-server detects.

As a user, I should be able to upload up to two videos before requiring registration.
Unregistered users should have their videos persisted for up to 48 hours before being
deleted. Registered users should have their videos/audio persisted for up to 90 days.

As a user, I should be able to view the subtitles for my uploads. They should be easy
to read. I should also be able to download .srt files, or other similar file formats.
I should also be able to add subtitles back to the video file. Or add an option in my
user settings that automatically adds subtitles back to my video so that I can download
my video with subtitles included in it.

## Non-Functional Requirements

Everything about this application needs to be fast. As a user, I should get very fast
visual feedback that my upload is happening. I should get interesting information that
indicates text from my upload is getting transcribed. I should have the most accurate
transcription possible.

### Accuracy Evaluation Plan

To evaluate transcription accuracy, we measure both word accuracy and timing alignment.

1. **Create a test corpus**:
   - 10-20 diverse audio/video samples (accents, noise levels, domains)
   - Ground truth transcripts with precise timestamps for each
   - Include edge cases: numbers, technical terms, music with background
   - See [Clean Room Evaluation](evaluation-cleanroom.md) for test framework

2. **Measure Word Error Rate (WER)**:
   - WER = (Substitutions + Insertions + Deletions) / Total Reference Words
   - Industry standard benchmark metric
   - Baseline comparison: Whisper Large V3 achieves ~7.88% WER

3. **Measure Timing Alignment**:
   - **Onset Error**: Difference between predicted and actual speech start time
   - **Offset Error**: Difference between predicted and actual speech end time
   - **Mean Absolute Error (MAE)**: Average timing error in milliseconds
   - **Early Start Penalty**: Subtitles appearing before speech are worse for UX
   - Target: MAE < 200ms, onset error < 100ms

4. **Speed metrics**:
   - Real-time factor (RTF): Processing time / Audio duration
   - Whisper Large V3: ~10-30 min per hour of audio
   - Whisper Turbo: 6x faster with ~1-2% accuracy loss

5. **Model comparison**:
   - large-v3 vs large-v3-turbo vs medium
   - Processing time per file
   - WER per file
   - Timing alignment per file

See [benchmarks](#competitor-benchmarks) in competitors section for competitor numbers.

## Design

The initial website should have a simple aesthetic. Consider https://www.anthropic.com/
or https://ampcode.com/ for inspiration. Over time, we should experiment with other
designs and see what resonates more with customers.

## Architecture

The frontend should be in Astro. It should have a proxy config to pass /api/* requests to
the backend. The frontend should forward all console.log, console.warning, etc messages
to the backend to make debugging easier when run with `npm run dev`. That should not happen
in production.

The backend should be written in Go. The backend should either use a remote whisper-server
or it should run whisper-server itself. Tests should use a smaller, faster model.
Production should use a larger, more accurate model. Cost vs speed tradeoff is still TBD.
Default to using the fastest whisper model you can. Allow for overriding the whisper model
via an env var. Allow for overriding the whisper server so we can run whisper server on
a baremetal Mac Mini. See [deployment.md](deployment.md) for whisper-server passthrough.
If running the whisper server process, then pipe all whisper-server logs to the backend logs
so its easier for debugging. Make sure to document all useful env vars in `backend/README.md`

Whisper cpp's source code is available in https://github.com/ggml-org/whisper.cpp . I
probably checked it out locally at ~/Github/whisper.cpp/. You may want to pull the latest
trunk/main/master branch. You may need to download new model files. Check
`pub_musings/cc_plugins/skills/*` for how to run whisper-cli locally and to add subtitles
to a video. Add documentation for what's useful.

File storage should be encrypted at rest using `age`. Max file size should be 500 MB to
start. The database should be sqlite. Authentication should be based on email + password.
As a user, I should be able to add 2-factor auth to my account via apps like Google
Authenticator on my phone. Bonus points if I can use passkeys.

Email service should be Resend API. You are blocked on me adding a real Resend API key.
You should search for and use a good Resend mock library for unit tests. If you can't
find one, then you should write one. You should similarly search or write a mock
implementation that can be used in integration tests.

Every third party API we add should have a mock library for unit tests and a mock
implementation for integration tests. Some API providers provide "sandbox" environments
that are useful to include in our [documentation](#Documentation).

Environment variables should be encrypted with `sops`.

Deploys are TBD. Implementing requires human intervention. See [deployment.md](deployment.md)
for deployment plan. I plan to run the website on a Mac Mini in a qemu VM, and ideally in a
docker container inside of the VM. See [metal-moltenvk.md](metal-moltenvk.md) for GPU
passthrough research. See [webhook-deployer.md](webhook-deployer.md) for automated deployment
integration leveraging `pub_musings/webhook-deployer/` for frontend and backend deploys.

Production deploys will leverage astro built static files with Caddy as the frontend load
balancer. Caddy can route all /api/* requests to the backend.

## Known Transcripts and Script Normalization

### The Problem

Whisper and other transcription engines often output text in romanized/transliterated form
rather than native scripts. For example:
- Hindi audio may be transcribed as "devanagari" instead of "देवनागरी"
- Japanese may come out as romaji ("arigatou") instead of hiragana/kanji ("ありがとう")
- Arabic may be romanized instead of using Arabic script

Similarly, users may paste known lyrics or transcripts that are already transliterated
(common on lyrics websites) when native script subtitles would be more appropriate for
language learners.

### Multi-Script Languages

Some languages have multiple valid writing systems:
- **Urdu/Hindi**: Mutually intelligible spoken languages, but Urdu uses Nastaliq (Arabic-derived)
  script while Hindi uses Devanagari
- **Serbian**: Uses both Cyrillic and Latin scripts
- **Japanese**: Uses Hiragana, Katakana, and Kanji (often mixed)
- **Chinese**: Simplified vs Traditional characters
- **Punjabi**: Gurmukhi (India) vs Shahmukhi (Pakistan)

### Backend: Script Conversion Library

The backend should include a script conversion/transliteration library that can:
1. Detect the current script of input text
2. Convert between scripts for the same language (e.g., romanized → Devanagari)
3. Handle mixed-script input gracefully
4. Preserve timing information when converting subtitle segments

Potential libraries to evaluate:
- **ICU (International Components for Unicode)**: Comprehensive transliteration support
- **Aksharamukha**: Supports 100+ scripts, especially strong for Indic languages
- **OpenCC**: Chinese simplified ↔ traditional conversion
- **Language-specific libraries**: polyglot, indic-transliteration, etc.

### Frontend: Language and Script Selection

The upload/edit interface should allow users to:
1. **Specify source language**: What language is being spoken in the video
2. **Choose target script**: Which writing system to use for subtitles
   - Show only valid scripts for the selected language
   - Default to the most common native script
3. **Request re-transliteration**: Convert existing subtitles to a different script

Example UI flow:
```
Language: Hindi
Script:   ○ Devanagari (देवनागरी) [default]
          ○ Romanized (IAST)
          ○ Romanized (casual)
```

### Quality Considerations

- Romanized → native script conversion is lossy in some cases (ambiguous spellings)
- Some content is intentionally romanized (song lyrics for international audiences)
- Consider showing confidence scores or highlighting uncertain conversions
- Allow manual correction of script conversion errors in the subtitle editor

## Competitors

### Key Players

| Service | AI Pricing | Human Pricing | Accuracy | Speed |
|---------|-----------|---------------|----------|-------|
| Rev | $0.25/min | $1.50/min | 90% AI, 99% human | 5 min AI, 12hr human |
| Happy Scribe | $5/hour | varies | Good | Fast |
| GoTranscript | $0.02/min | $1.02-2.34/min | 99%+ human | Fast AI |
| Otter.ai | Free tier, $8.33/mo Pro | N/A | Good | Real-time |
| Sonix | $5/hour | N/A | Good | Fast |
| VEED | SaaS pricing | N/A | Good | Seconds |
| Descript | SaaS pricing | N/A | Good | Fast |

### Competitor Benchmarks

Word Error Rate (WER) comparison (lower is better):
- **Assembly AI Universal-2**: 6.68% WER - current best commercial
- **Whisper Large V3**: 7.88% WER - our baseline model
- **Whisper Turbo**: 7.75% WER, 6x faster than Large V3
- **NVIDIA Canary Qwen 2.5B**: Best open-source accuracy
- **Granite-Speech-3.3**: 8.18% WER (edge deployment)
- **Distil-Whisper**: 14.93% WER, 6x faster, 756M params

### Competitive Advantages We Can Offer

1. **Music video focus**: Optimized for song lyrics alignment and timing
2. **Language learning focus**: Native script output (Devanagari, Kanji, etc.)
3. **Per-minute pricing**: Matches industry expectations for SaaS customers
4. **Open-source models**: Whisper is free to run, can swap to better models
5. **Subtitle editing**: Built-in correction and timing adjustment
6. **Known lyrics alignment**: Upload lyrics to fix transcription automatically

### Areas to Improve

1. Speed: Whisper Large is slow (~10-30min/hour). Consider Turbo or Distil.
2. Accuracy: Consider supporting newer models (Canary, Granite) as alternatives.
3. Multi-speaker diarization: Not currently supported.

## Documentation

Write documentation after every meaningful change. If documentation gets too big, then
move old documentation out of frequently checked docs and add links to it. Make .md files
link to each other.
