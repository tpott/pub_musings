# 007_MULTIPLE_FORMATS

## Goal
Add support for multiple transcription output formats including VTT and embedded video with subtitles.

## Current State
- `internal/transcribe/service.go` already supports text, json, srt, vtt formats via whisper.cpp
- Worker hardcodes SRT format (worker.go:107)
- `/api/transcribe` endpoint doesn't accept format parameter
- `/api/upload` endpoint doesn't accept format parameter
- No embedded video functionality

## Implementation Plan

### Step 1: Add Format Parameter to /api/transcribe
- Accept `format` query parameter (default: srt)
- Validate format (srt, vtt, text, json)
- Pass format to TranscribeFile()
- Update response to include format used

### Step 2: Add Format Parameter to /api/upload
- Accept `format` form field (default: srt)
- Store format in Job.OutputFormat
- Worker uses job.OutputFormat instead of hardcoded SRT

### Step 3: Add Embedded Video Format
- Add FormatEmbedded constant
- Implement ffmpeg wrapper for subtitle burning
- Generate .mp4 file with subtitles overlay
- Handle both video and audio input files

### Step 4: Testing
- Unit test format parameter parsing
- Integration test for VTT format
- Integration test for embedded format (requires ffmpeg)
- Update Playwright tests if needed

## Acceptance Criteria
- `curl -F 'file=@test.mp3' localhost:8080/api/transcribe?format=vtt` returns VTT
- `curl -F 'file=@test.mp3' localhost:8080/api/transcribe?format=srt` returns SRT (default)
- `curl -F 'file=@test.mp4' -F 'format=embedded' localhost:8080/api/upload` creates video with subtitles
- Worker respects job.OutputFormat field
- Tests pass

## Files to Modify
- `backend/cmd/server/transcribe_handlers.go` - Add format parameter
- `backend/cmd/server/upload_handlers.go` - Add format parameter
- `backend/internal/transcribe/service.go` - Add embedded format support
- `backend/internal/transcribe/embed.go` - New file for ffmpeg integration
- `backend/internal/worker/worker.go` - Use job.OutputFormat
- Tests as needed

## Dependencies
- ffmpeg must be installed for embedded format

## Notes
- VTT format is already supported by whisper.cpp, just need to wire it up
- Embedded format is more complex, may require separate implementation
- Should validate that embedded format only works with video files
