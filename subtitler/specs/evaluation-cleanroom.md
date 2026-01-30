# Clean Room Model Evaluation Framework

This document describes the framework for objectively evaluating transcription model accuracy and performance.

## Overview

A "clean room" evaluation compares multiple Whisper models without bias:
- **large-v3** - Most accurate, slowest
- **large-v3-turbo** - 6x faster, ~1-2% accuracy loss
- **medium** - Smaller, faster, less accurate

## Test Corpus

### Requirements

1. **Reproducible** - Test audio generated from seed data (committed to repo)
2. **Diverse** - Multiple languages, accents, domains
3. **Ground truth** - Exact transcripts known (we generate the audio)
4. **Not committed** - Audio files generated on demand (too large for git)

### Using Text-to-Speech (TTS)

Generate test audio using open-source TTS models:

**Recommended:** [Coqui TTS](https://github.com/coqui-ai/TTS) or [Piper](https://github.com/rhasspy/piper)

```bash
# Install Piper (fast, local TTS)
pip install piper-tts

# Generate audio from text
echo "Hello, this is a test of the transcription system." | \
  piper --model en_US-lessac-medium --output_file test.wav
```

### Seed Data Format

Store seed data in `evaluation/seeds/`:

```yaml
# evaluation/seeds/english-basic.yaml
id: english-basic
language: en
voice: en_US-lessac-medium
segments:
  - text: "Hello, this is a test of the transcription system."
    expected_duration: 3.5
  - text: "The quick brown fox jumps over the lazy dog."
    expected_duration: 2.8
```

### Generation Script

```bash
# evaluation/generate.sh
#!/bin/bash
# Generates test audio from seed files
# Output: evaluation/audio/ (gitignored)

for seed in evaluation/seeds/*.yaml; do
  # Parse YAML and generate audio with TTS
  # ...
done
```

## Metrics

### 1. Word Error Rate (WER)

```
WER = (Substitutions + Insertions + Deletions) / Total Reference Words × 100
```

**Example:**
- Reference: "Hello world"
- Hypothesis: "Hello word"
- WER = 1/2 = 50%

### 2. Timing Alignment

| Metric | Description | Target |
|--------|-------------|--------|
| **Onset Error** | subtitle_start - actual_speech_start | < 100ms |
| **Offset Error** | subtitle_end - actual_speech_end | < 200ms |
| **Mean Absolute Error** | Average of all timing errors | < 200ms |
| **Early Start Rate** | % of subtitles appearing before speech | < 5% |

**Why timing matters:**
- Subtitles appearing before speech spoil the content
- Late subtitles make it hard to follow along
- For music videos, precise timing is critical for karaoke-style display

### 3. Processing Speed

| Metric | Description |
|--------|-------------|
| **Real-Time Factor (RTF)** | processing_time / audio_duration |
| **Throughput** | Minutes of audio per minute of processing |

RTF < 1.0 means faster than real-time.

## Evaluation Script

```bash
# evaluation/evaluate.py
#!/usr/bin/env python3
"""
Evaluate Whisper models against test corpus.

Usage:
  python evaluation/evaluate.py --model large-v3
  python evaluation/evaluate.py --model large-v3-turbo
  python evaluation/evaluate.py --all
"""

import argparse
from whisper_eval import calculate_wer, measure_timing, run_transcription

def evaluate_model(model_name, audio_files, ground_truth):
    results = []
    for audio, truth in zip(audio_files, ground_truth):
        start = time.time()
        hypothesis = run_transcription(model_name, audio)
        elapsed = time.time() - start

        wer = calculate_wer(truth.text, hypothesis.text)
        timing = measure_timing(truth.segments, hypothesis.segments)

        results.append({
            'file': audio,
            'wer': wer,
            'onset_error_ms': timing.onset_error,
            'offset_error_ms': timing.offset_error,
            'processing_time': elapsed,
            'audio_duration': truth.duration,
            'rtf': elapsed / truth.duration
        })
    return results

def print_summary(model_name, results):
    avg_wer = sum(r['wer'] for r in results) / len(results)
    avg_onset = sum(r['onset_error_ms'] for r in results) / len(results)
    avg_rtf = sum(r['rtf'] for r in results) / len(results)

    print(f"\n=== {model_name} ===")
    print(f"Average WER: {avg_wer:.2f}%")
    print(f"Average Onset Error: {avg_onset:.0f}ms")
    print(f"Average RTF: {avg_rtf:.2f}x")
```

## Directory Structure

```
evaluation/
├── README.md              # How to run evaluations
├── seeds/                 # YAML seed files (committed)
│   ├── english-basic.yaml
│   ├── english-music.yaml
│   ├── hindi-basic.yaml
│   └── ...
├── generate.sh            # Generate audio from seeds
├── evaluate.py            # Run evaluation
├── requirements.txt       # Python dependencies
├── audio/                 # Generated audio (gitignored)
└── results/               # Evaluation results (committed)
    └── 2024-01-15.json
```

## Running Evaluations

```bash
# 1. Generate test audio (first time or after seed changes)
cd evaluation
./generate.sh

# 2. Run evaluation for all models
python evaluate.py --all

# 3. Run evaluation for specific model
python evaluate.py --model large-v3-turbo

# 4. View results
cat results/latest.json
```

## Expected Results

Based on published benchmarks:

| Model | WER | RTF | Notes |
|-------|-----|-----|-------|
| large-v3 | ~7.9% | 0.3-0.5 | Most accurate |
| large-v3-turbo | ~7.8% | 0.05-0.1 | 6x faster |
| medium | ~10-12% | 0.02-0.05 | Fastest |

## Legal Considerations

Using TTS-generated audio ensures:
1. No copyright issues
2. Reproducible test corpus
3. Exact ground truth known
4. Can share methodology publicly

## Future Improvements

1. Add real recorded samples (with permission)
2. Test non-English languages
3. Add background noise variants
4. Test music+speech mixing
5. Measure diarization accuracy (if supported)

## See Also

- [subtitler.md](subtitler.md) - Accuracy evaluation plan
- [lyrics-alignment.md](lyrics-alignment.md) - Known lyrics alignment algorithm
