# Language Detection Specification

## Overview

Improve language auto-detection before transcription by checking multiple sources for language hints. This helps users avoid incorrect automatic language detection and provides confidence indicators.

## Motivation

Currently, users can:
1. Select a language manually from a dropdown before transcription
2. Rely on Whisper's automatic language detection

Problems with current approach:
- Auto-detection sometimes guesses wrong, especially for mixed-language content
- Users don't know the detected language until after transcription completes
- No pre-transcription hints to help users make better choices

## Detection Sources

### 1. Video Metadata (ffprobe)

Use ffprobe to extract language metadata from audio tracks.

**ffprobe command:**
```bash
ffprobe -v error -select_streams a:0 -show_entries stream_tags=language -of json <file>
```

**Expected output:**
```json
{
    "streams": [
        {
            "tags": {
                "language": "eng"
            }
        }
    ]
}
```

**Notes:**
- Language tags use ISO 639-2 (3-letter codes like "eng", "spa", "deu")
- Convert to ISO 639-1 (2-letter codes) for Whisper compatibility
- Not all videos have language metadata (especially user-created content)
- MKV files are more likely to have metadata than MP4

### 2. Filename Patterns

Common filename patterns that indicate language:
- ISO 639-1 codes: `video.en.mp4`, `video_en.mp4`, `video-en.mp4`
- ISO 639-2 codes: `video.eng.mp4`, `video_eng.mp4`
- Language names: `video_english.mp4`, `video-spanish.mp4`
- Brackets/parentheses: `video (English).mp4`, `video [en].mp4`
- Common patterns from media tools: `S01E01.720p.eng.mp4`

**Detection regex patterns:**
```regex
# ISO codes before extension
[._-](en|es|fr|de|it|pt|ru|zh|ja|ko|ar|hi)\.
[._-](eng|spa|fra|deu|ita|por|rus|zho|jpn|kor|ara|hin)\.

# Language names before extension
[._-](english|spanish|french|german|italian|portuguese|russian|chinese|japanese|korean|arabic|hindi)[._-]?\.

# Bracketed codes
\[(en|es|fr|de|it|pt|ru|zh|ja|ko|ar|hi)\]
\((en|es|fr|de|it|pt|ru|zh|ja|ko|ar|hi)\)
```

**Confidence levels:**
- High: ISO code directly before extension (e.g., `video.en.mp4`)
- Medium: Language name in filename (e.g., `video_english.mp4`)
- Low: ISO code anywhere in filename (e.g., `english_lesson_01.mp4`)

### 3. LLM API (Optional, Future Enhancement)

For cases where metadata and filename don't provide hints, an optional LLM API could analyze:
- Video title/description (if available)
- First few seconds of transcribed audio (using fast, low-quality whisper pass)
- Visual text detection (OCR on video frames)

**NOT IMPLEMENTED IN PHASE 1** - Requires `LANGUAGE_DETECT_API_KEY` and adds latency/cost.

## API Changes

### New Endpoint: GET /api/videos/{id}/language-hints

Returns language detection results from multiple sources.

**Response:**
```json
{
    "hints": [
        {
            "source": "metadata",
            "language": "en",
            "language_name": "English",
            "confidence": "high",
            "raw_value": "eng"
        },
        {
            "source": "filename",
            "language": "en",
            "language_name": "English",
            "confidence": "medium",
            "raw_value": "english"
        }
    ],
    "suggested_language": "en",
    "suggested_confidence": "high"
}
```

**Fields:**
- `hints`: Array of all detected hints with their sources
- `source`: Where hint came from (`metadata`, `filename`, `llm`)
- `language`: ISO 639-1 code for Whisper
- `language_name`: Human-readable name
- `confidence`: `high`, `medium`, or `low`
- `raw_value`: Original value before normalization
- `suggested_language`: Best guess (highest confidence)
- `suggested_confidence`: Confidence of suggestion

### Modified Endpoint: POST /api/upload

Add language hints to upload response after processing.

**Current response (partial):**
```json
{
    "id": "abc123",
    "filename": "video.mp4",
    "status": "success"
}
```

**New response fields:**
```json
{
    "id": "abc123",
    "filename": "video.mp4",
    "status": "success",
    "language_hints": {
        "hints": [...],
        "suggested_language": "en",
        "suggested_confidence": "medium"
    }
}
```

## Backend Implementation

### Package: `backend/language`

```go
package language

// LanguageHint represents a detected language indicator from a source.
type LanguageHint struct {
    Source      string `json:"source"`       // "metadata", "filename", "llm"
    Language    string `json:"language"`     // ISO 639-1 code
    LanguageName string `json:"language_name"` // Human-readable name
    Confidence  string `json:"confidence"`   // "high", "medium", "low"
    RawValue    string `json:"raw_value"`    // Original value before normalization
}

// DetectionResult contains all hints and a suggestion.
type DetectionResult struct {
    Hints              []LanguageHint `json:"hints"`
    SuggestedLanguage  string         `json:"suggested_language"`
    SuggestedConfidence string         `json:"suggested_confidence"`
}

// DetectFromMetadata extracts language from video metadata using ffprobe.
func DetectFromMetadata(filePath string) (*LanguageHint, error)

// DetectFromFilename extracts language from filename patterns.
func DetectFromFilename(filename string) (*LanguageHint, error)

// Detect runs all detection methods and returns combined results.
func Detect(filePath, filename string) (*DetectionResult, error)
```

### ISO 639-2 to 639-1 Conversion

Common mappings needed:
```go
var iso639_2to1 = map[string]string{
    "eng": "en", "spa": "es", "fra": "fr", "deu": "de",
    "ita": "it", "por": "pt", "rus": "ru", "zho": "zh",
    "jpn": "ja", "kor": "ko", "ara": "ar", "hin": "hi",
    "nld": "nl", "pol": "pl", "tur": "tr", "vie": "vi",
    "tha": "th", "ind": "id", "ukr": "uk", "swe": "sv",
    "und": "",  // Undetermined
}
```

### Language Name Display

Map language codes to display names:
```go
var languageNames = map[string]string{
    "en": "English", "es": "Spanish", "fr": "French", "de": "German",
    "it": "Italian", "pt": "Portuguese", "ru": "Russian", "zh": "Chinese",
    "ja": "Japanese", "ko": "Korean", "ar": "Arabic", "hi": "Hindi",
    "nl": "Dutch", "pl": "Polish", "tr": "Turkish", "vi": "Vietnamese",
    "th": "Thai", "id": "Indonesian", "uk": "Ukrainian", "sv": "Swedish",
}
```

## Frontend Implementation

### Upload Page Changes

After file selection (before upload starts):
1. Show "Detecting language..." indicator
2. After upload completes, check response for `language_hints`
3. If hints found:
   - Show detected language with confidence badge
   - Pre-select suggested language in dropdown
   - Allow user to override before transcription

**UI Example:**
```
Selected: video_english.mp4

Language detected: English (medium confidence)
  ⓘ Based on: filename pattern "english"

[  Auto-detect  ▾]  ← Dropdown pre-selects detected language
                     with "Auto-detect" still as first option
```

### Confidence Badges

- **High confidence** (green): Metadata tag present
- **Medium confidence** (yellow): Filename pattern match
- **Low confidence** (gray): Weak pattern match

## Testing

### Unit Tests

1. **ISO 639-2 to 639-1 conversion**
   - Test common codes (eng→en, spa→es, etc.)
   - Test unknown codes return empty string
   - Test case insensitivity

2. **Filename pattern detection**
   - Test `video.en.mp4` → high confidence, "en"
   - Test `video_english.mp4` → medium confidence, "en"
   - Test `video[es].mp4` → high confidence, "es"
   - Test `my_video.mp4` → no hint
   - Test `english_lesson.mp4` → low confidence, "en" (false positive possible)

3. **Metadata extraction**
   - Mock ffprobe output
   - Test missing language tag
   - Test "und" (undetermined) handling

### Integration Tests

1. Upload video with language metadata, verify hints returned
2. Upload video with language in filename, verify hints returned
3. Upload video with no hints, verify empty result
4. Verify frontend displays hints correctly

## Configuration

No new environment variables required for Phase 1.

**Future (Phase 2):**
- `LANGUAGE_DETECT_API_KEY`: API key for LLM-based detection
- `LANGUAGE_DETECT_TIMEOUT`: Timeout for LLM API calls (default: 5s)

## Security Considerations

- Filename patterns are checked with regex - no execution risk
- ffprobe is already trusted (used for video validation)
- No user input is passed to shell commands

## Implementation Phases

### Phase 1 (Completed)
- [x] Create specs/language-detection.md
- [x] Create `backend/language` package
- [x] Implement `DetectFromMetadata()` using ffprobe
- [x] Implement `DetectFromFilename()` using regex
- [x] Add language hints to upload response
- [x] Create `GET /api/videos/{id}/language-hints` endpoint
- [x] Update frontend to display hints
- [x] Add unit tests (136+ tests in language package and API)

### Phase 2 (Future)
- [ ] Add LLM API integration for uncertain cases
- [ ] Add caching for repeated requests
- [ ] Add more filename patterns based on user feedback
- [ ] Support additional audio track metadata fields

## References

- [ISO 639-1 Code List](https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes)
- [ISO 639-2 Code List](https://en.wikipedia.org/wiki/List_of_ISO_639-2_codes)
- [Whisper Language Support](https://github.com/openai/whisper#available-models-and-languages)
- [ffprobe Documentation](https://ffmpeg.org/ffprobe.html)
