# Embedded Subtitle Detection and Comparison

This spec describes the feature for detecting embedded subtitle tracks in uploaded videos and optionally comparing them against Whisper-generated transcriptions.

## Overview

Many video files contain embedded subtitle tracks (soft subtitles). These could be:
- Professional subtitles added during post-production
- User-added subtitles from other transcription tools
- Fan-created subtitles for foreign content

Detecting these subtitles provides value by:
1. Alerting users that subtitles already exist
2. Allowing users to extract existing subtitles without re-transcription
3. Enabling comparison to assess Whisper transcription quality

## Implementation Plan

### Phase 1: Detection (This Task)

**Backend Changes:**

1. Add `GetSubtitleTracks()` to `backend/audio/audio.go`:
   - Uses ffprobe to detect subtitle streams
   - Returns list of subtitle tracks with metadata (index, language, codec, title)

2. Add `SubtitleTrack` struct:
   ```go
   type SubtitleTrack struct {
       Index    int    `json:"index"`
       Language string `json:"language"` // ISO 639-1/2 code
       Title    string `json:"title"`    // Optional track title
       Codec    string `json:"codec"`    // e.g., "subrip", "ass", "mov_text"
       Default  bool   `json:"default"`  // Is this the default track?
       Forced   bool   `json:"forced"`   // Is this a forced subtitle track?
   }
   ```

3. Store detected subtitle tracks in database:
   - Add `embedded_subtitles_json` column to `videos` table
   - Populate during upload processing

4. Return subtitle track info in video API responses:
   - Include `embedded_subtitles` array in `GET /api/videos/{id}`

**Frontend Changes:**

1. Show badge/indicator when embedded subtitles exist
2. Display detected track information (language, type)

### Phase 2: Extraction (Future)

1. Add `ExtractSubtitleTrack()` function:
   ```go
   ExtractSubtitleTrack(videoPath string, trackIndex int, outputPath string) error
   ```

2. Add endpoint `GET /api/videos/{id}/embedded-subtitles/{track}`:
   - Extracts specified track as SRT/VTT
   - Caches result for future requests

3. Frontend "Use embedded subtitles" button:
   - Replaces Whisper transcription with embedded track

### Phase 3: Comparison (Future)

1. Add subtitle comparison metrics:
   ```go
   type SubtitleComparison struct {
       WER            float64 `json:"wer"`             // Word Error Rate
       CER            float64 `json:"cer"`             // Character Error Rate
       SegmentCount   int     `json:"segment_count"`   // Number of segments compared
       TimingDrift    float64 `json:"timing_drift_ms"` // Average timing difference
       MismatchCount  int     `json:"mismatch_count"`  // Segments with significant differences
   }
   ```

2. Add endpoint `POST /api/videos/{id}/compare-subtitles`:
   - Compares Whisper output against embedded track
   - Returns comparison metrics

3. Frontend comparison view:
   - Side-by-side diff view
   - Highlight timing differences
   - Show overall accuracy score

---

## Technical Details

### ffprobe Command for Subtitle Detection

```bash
ffprobe -v error \
  -select_streams s \
  -show_entries stream=index,codec_name:stream_tags=language,title \
  -of json \
  input.mp4
```

Example output:
```json
{
    "streams": [
        {
            "index": 2,
            "codec_name": "subrip",
            "tags": {
                "language": "eng",
                "title": "English"
            }
        },
        {
            "index": 3,
            "codec_name": "ass",
            "tags": {
                "language": "jpn",
                "title": "Japanese"
            }
        }
    ]
}
```

### ffmpeg Command for Subtitle Extraction

```bash
# Extract to SRT format
ffmpeg -i input.mp4 -map 0:s:0 -c:s srt output.srt

# Extract specific stream by index
ffmpeg -i input.mp4 -map 0:2 output.srt
```

### Subtitle Codecs

Common embedded subtitle formats:
| Codec | Format | Text-based |
|-------|--------|------------|
| `subrip` | SRT | Yes |
| `ass` | Advanced SubStation Alpha | Yes |
| `mov_text` | MP4 text track | Yes |
| `webvtt` | WebVTT | Yes |
| `hdmv_pgs_subtitle` | Blu-ray PGS | No (image-based) |
| `dvd_subtitle` | DVD VOB | No (image-based) |
| `dvb_subtitle` | DVB | No (image-based) |

Image-based subtitles cannot be extracted to SRT format and require OCR.

### Database Schema Changes

Migration to add embedded subtitles column:

```sql
-- Migration 006: Add embedded subtitles column
ALTER TABLE videos ADD COLUMN embedded_subtitles_json TEXT DEFAULT NULL;
```

JSON structure stored:
```json
{
    "tracks": [
        {
            "index": 2,
            "language": "eng",
            "title": "English",
            "codec": "subrip",
            "default": true,
            "forced": false
        }
    ],
    "detected_at": "2026-01-27T12:00:00Z"
}
```

---

## API Changes

### GET /api/videos/{id}

Response now includes:
```json
{
    "id": "abc123...",
    "filename": "video.mp4",
    "embedded_subtitles": [
        {
            "index": 2,
            "language": "eng",
            "title": "English",
            "codec": "subrip",
            "default": true,
            "forced": false,
            "text_based": true
        }
    ],
    ...
}
```

### GET /api/videos/{id}/embedded-subtitles/{track} (Phase 2)

Query parameters:
- `format`: `srt` (default), `vtt`, `json`

Response: Subtitle file content with appropriate Content-Type.

### POST /api/videos/{id}/compare-subtitles (Phase 3)

Request:
```json
{
    "track_index": 2
}
```

Response:
```json
{
    "wer": 0.05,
    "cer": 0.03,
    "segment_count": 150,
    "timing_drift_ms": 42.5,
    "mismatch_count": 8,
    "details": [
        {
            "whisper": "Hello world",
            "embedded": "Hello, world!",
            "start_ms": 1500,
            "drift_ms": 50
        }
    ]
}
```

---

## Testing

### Unit Tests

1. `TestGetSubtitleTracks_NoSubtitles` - Video without embedded subtitles
2. `TestGetSubtitleTracks_SingleTrack` - Video with one subtitle track
3. `TestGetSubtitleTracks_MultipleTracks` - Video with multiple languages
4. `TestGetSubtitleTracks_ImageBasedSubtitles` - Blu-ray/DVD style subtitles
5. `TestGetSubtitleTracks_FfprobeNotAvailable` - Graceful fallback

### Integration Tests

1. Test upload flow detects embedded subtitles
2. Test API returns subtitle track information
3. Test extraction produces valid SRT (Phase 2)
4. Test comparison metrics calculation (Phase 3)

### Test Files

Create minimal test video files:
1. `test_no_subtitles.mp4` - Plain video
2. `test_srt_subtitle.mkv` - MKV with SRT track
3. `test_multi_lang.mkv` - Multiple language tracks

---

## UI Mockups

### Upload Page (After Transcription)

```
┌────────────────────────────────────────────────────────┐
│ ✅ Transcription complete                              │
│                                                        │
│ ℹ️ This video has embedded subtitles:                  │
│   • English (SRT) - Default                            │
│   • Japanese (ASS)                                     │
│                                                        │
│ [Use Whisper] [Use Embedded ▼]                         │
│                                                        │
│ Current: Whisper transcription (150 segments)          │
└────────────────────────────────────────────────────────┘
```

### My Videos Page

```
┌───────────────────┬───────────────────────────────────┐
│                   │ video.mp4                         │
│   [Thumbnail]     │ 10:32 • Uploaded 2h ago           │
│                   │ 📝 Has embedded subtitles (EN)    │
│                   │                                   │
│                   │ [View] [Download ▼] [Delete]      │
└───────────────────┴───────────────────────────────────┘
```

---

## Implementation Status

- [x] Phase 1: Detection
  - [x] Add `GetSubtitleTracks()` to audio.go
  - [x] Add SubtitleTrack struct
  - [x] Add database migration (006_add_embedded_subtitles)
  - [x] Update Video struct to include embedded subtitles
  - [x] Call detection during upload (both simple and chunked)
  - [x] Include in API responses (GET /api/videos with parsed JSON)
  - [x] Frontend indicator/badge on My Videos page
- [x] Phase 2: Extraction
  - [x] Add `ExtractSubtitleTrack()` function to audio.go
  - [x] Add `GET /api/videos/{id}/embedded-subtitles/{track}` endpoint
  - [x] Include embedded subtitles in transcription status response
  - [x] Frontend "Use embedded subtitles" notice and button on upload page
  - [x] SRT parsing to load extracted subtitles into editor
- [ ] Phase 3: Comparison
  - [ ] Implement WER/CER calculation
  - [ ] Add comparison endpoint
  - [ ] Frontend diff view

---

## Security Considerations

1. **Path traversal**: Validate track index is within bounds
2. **Resource limits**: Cap number of subtitle tracks processed
3. **Codec filtering**: Only allow text-based codecs for extraction
4. **Output sanitization**: Escape subtitle content in API responses
