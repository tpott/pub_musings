# Evaluation Framework

This directory contains tools for evaluating Whisper transcription accuracy using a reproducible test corpus.

## Overview

The evaluation framework:
1. Generates test audio from seed text files using Piper TTS
2. Runs transcription against whisper-server
3. Calculates Word Error Rate (WER), Character Error Rate (CER), and Real-Time Factor (RTF)
4. Saves results for comparison

## Prerequisites

- **Python 3.9+**
- **piper-tts** (system command `piper`) for audio generation
- **whisper-server** running locally for transcription evaluation

## Setup

### 1. Create Virtual Environment

```bash
cd evaluation
python3 -m venv venv
source venv/bin/activate
```

### 2. Install Dependencies

```bash
pip install -r requirements.txt
```

Dependencies:
- **piper-tts**: Fast, local text-to-speech (provides the `piper` CLI command)
- **PyYAML**: YAML parsing for seed files
- **requests**: HTTP client for whisper-server
- **jiwer**: Word/Character Error Rate calculation (optional, WER/CER skipped if missing)

### 3. Download Piper Voice Model

Piper needs a voice model. The default seed uses `en_US-lessac-medium`.
On first run, Piper may download the model automatically.

To download manually:
```bash
piper --model en_US-lessac-medium --download
```

### 4. Start whisper-server

The evaluation script expects whisper-server running on `http://localhost:8765`.

```bash
# Set environment variable if using a different URL
export WHISPER_URL=http://localhost:8765
```

**Note:** `evaluate.py --list` and `evaluate.py --help` work without
any dependencies installed or whisper-server running.

## Usage

### Generate Test Audio

```bash
# Generate audio for all seeds
./generate.sh

# Generate audio for specific seed
./generate.sh english-basic
```

Audio files are saved to `audio/<seed-id>/` and are gitignored.

### Run Evaluation

```bash
# List available seeds
python evaluate.py --list

# Evaluate all seeds
python evaluate.py

# Evaluate specific seed
python evaluate.py --seed english-basic

# Use different whisper-server
python evaluate.py --whisper-url http://192.168.1.100:8765
```

### View Results

```bash
# Latest results
cat results/latest.json

# All results (timestamped)
ls -la results/
```

## Test Corpus

### Seed File Format

Seeds are YAML files in `seeds/`:

```yaml
id: english-basic
language: en
voice: en_US-lessac-medium
description: "Basic English sentences"

segments:
  - id: simple-01
    text: "Hello, this is a test."
    expected_duration: 2.5
```

### Adding New Seeds

1. Create a new YAML file in `seeds/`
2. Run `./generate.sh <seed-id>` to generate audio
3. Run `python evaluate.py --seed <seed-id>` to evaluate

### Recommended Test Cases

- **Simple sentences**: Basic declarative statements
- **Numbers and dates**: "January 15th, 2026", "$1,234.56"
- **Technical terms**: API, HTTPS, database
- **Homophones**: they're/their/there, its/it's
- **Long sentences**: 20+ word sentences
- **Non-English**: Hindi, Spanish, etc.

## Metrics

### Word Error Rate (WER)

```
WER = (Substitutions + Insertions + Deletions) / Total Reference Words
```

Lower is better. Industry standard for ASR evaluation.

| WER | Quality |
|-----|---------|
| < 5% | Excellent |
| 5-10% | Good |
| 10-20% | Acceptable |
| > 20% | Poor |

### Real-Time Factor (RTF)

```
RTF = Processing Time / Audio Duration
```

RTF < 1.0 means faster than real-time.

| RTF | Speed |
|-----|-------|
| 0.05 | 20x faster than real-time |
| 0.1 | 10x faster than real-time |
| 0.5 | 2x faster than real-time |
| 1.0 | Real-time |

### Character Error Rate (CER)

Like WER but at character level. Useful for non-space-separated languages.

## Results Format

```json
{
  "timestamp": "2026-01-26T22:30:00",
  "whisper_url": "http://localhost:8765",
  "seeds": [
    {
      "summary": {
        "seed_id": "english-basic",
        "language": "en",
        "total_segments": 11,
        "successful_segments": 11,
        "average_wer": 0.03,
        "average_rtf": 0.08
      },
      "segments": [
        {
          "id": "simple-01",
          "status": "success",
          "reference": "Hello, this is a test...",
          "hypothesis": "Hello, this is a test...",
          "wer": 0.0,
          "rtf": 0.07
        }
      ]
    }
  ]
}
```

## Directory Structure

```
evaluation/
├── README.md           # This file
├── requirements.txt    # Python dependencies
├── generate.sh         # Generate audio from seeds
├── evaluate.py         # Run evaluation
├── seeds/              # Seed YAML files (committed)
│   └── english-basic.yaml
├── audio/              # Generated audio (gitignored)
└── results/            # Evaluation results
    └── latest.json
```

## Troubleshooting

### "piper not found"

Install piper-tts:
```bash
pip install piper-tts
```

### "Cannot connect to whisper-server"

Make sure whisper-server is running:
```bash
curl http://localhost:8765/health
```

### "Audio file not found"

Generate audio first:
```bash
./generate.sh
```

### Low Accuracy Results

1. Check audio quality (listen to generated files)
2. Try a different Piper voice
3. Check if whisper-server language detection is correct
4. Verify text doesn't contain unusual characters

## Limitations

- **No timing alignment metrics** - The spec ([evaluation-cleanroom.md](../specs/evaluation-cleanroom.md))
  describes onset/offset error and early start rate metrics, but these are not yet
  implemented. Only WER, CER, and RTF are currently measured.
- **English only** - Only an English seed corpus is provided. The framework supports
  other languages but no non-English seeds exist yet.
- **TTS-generated audio** - Evaluation uses synthetic speech, which may not reflect
  real-world accuracy on natural speech, music vocals, or noisy recordings.

## See Also

- [specs/evaluation-cleanroom.md](../specs/evaluation-cleanroom.md) - Design document
- [specs/subtitler.md](../specs/subtitler.md) - Main specification
