# Status

This file tracks high level progress on the peekaboo project.

## Current State

- **Go backend scaffolded** - main.go with /health endpoint, listens on :8080
- **Astro frontend scaffolded** - mobile-first layout with mic button and media display
- **Media assets sourced** - 6 animals with CC0 photos and audio via scripts/source-media.sh
- **SQLite database** - db package with concepts/media_sets tables, GetRandomMediaSet, tests pass
- **Age encryption** - crypto package with EncryptFile/DecryptFile using filippo.io/age
- **Whisper API** - api/transcribe.go forwards audio to whisper-server, returns transcript

## Last Completed

- Task 7: Whisper-server integration (2026-02-03)
- Task 6: Age encryption (2026-02-03)
- Task 5: SQLite media database (2026-02-03)
- Task 4: source-media.sh script (2026-02-03)
- Task 3: .gitignore (2026-02-03)
- Task 2: Astro frontend scaffold (2026-02-03)
- Task 1: Go backend scaffold (2026-02-03)
