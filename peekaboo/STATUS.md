# Status

This file tracks high level progress on the peekaboo project.

## Current State

- **Go backend scaffolded** - main.go with /health endpoint, listens on :8080
- **Astro frontend scaffolded** - mobile-first layout with mic button and media display
- **Media assets sourced** - 6 animals with CC0 photos and audio via scripts/source-media.sh
- **SQLite database** - db package with concepts/media_sets tables, GetRandomMediaSet, tests pass
- **Age encryption** - crypto package with EncryptFile/DecryptFile using filippo.io/age
- **Whisper API** - api/transcribe.go forwards audio to whisper-server, returns transcript
- **LLM Intent API** - api/intent.go extracts subject from voice commands using Anthropic tool calls
- **Media lookup API** - api/media.go returns random media set for a concept
- **Frontend mic recording** - MicButton with MediaRecorder, sends audio to /api/transcribe

## Last Completed

- Task 10: Frontend microphone recording with MediaRecorder (2026-02-04)
- Task 9: Media lookup API GET /api/media/{concept} (2026-02-04)
- Task 8: LLM intent recognition with Anthropic tool calls (2026-02-04)
- Task 7: Whisper-server integration (2026-02-03)
- Task 6: Age encryption (2026-02-03)
- Task 5: SQLite media database (2026-02-03)
- Task 4: source-media.sh script (2026-02-03)
- Task 3: .gitignore (2026-02-03)
- Task 2: Astro frontend scaffold (2026-02-03)
- Task 1: Go backend scaffold (2026-02-03)
