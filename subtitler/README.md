# Subtitler

A web-based subtitling service that allows users to upload video/audio files and receive transcribed subtitles or files with embedded subtitles. Built using whisper.cpp for transcription.

## Overview

Subtitler provides:
- Automatic transcription of audio and video files using whisper.cpp
- Multiple output formats (SRT, VTT, embedded video)
- User accounts and job history
- Background processing with notifications

## Tech Stack

- **Frontend**: Astro (TypeScript)
- **Backend**: Go
- **Transcription**: whisper.cpp (medium model)
- **Database**: SQLite
- **File Storage**: Local, encrypted with age
- **Email**: Resend API

## Project Structure

```
subtitler/
├── frontend/           # Astro web application
│   ├── src/
│   └── package.json
├── backend/            # Go API server
│   ├── cmd/server/     # Main server entry point
│   ├── internal/       # Internal packages
│   │   ├── auth/       # Authentication logic
│   │   ├── transcribe/ # Transcription integration
│   │   └── storage/    # File storage
│   └── go.mod
├── v1/                 # Previous Python implementation (archived)
└── deploy/             # Deployment configurations (future)
```

## Getting Started

See [INSTALL.md](INSTALL.md) for detailed setup instructions.

Quick start:
```bash
# Frontend
cd frontend && npm install && npm run dev

# Backend (in separate terminal)
cd backend && go run ./cmd/server
```

## Documentation

- [INSTALL.md](INSTALL.md) - Setup and installation guide
- [001_RALPH_SUBTITLER.md](001_RALPH_SUBTITLER.md) - Architecture and planning
- [TASKS.jsonl](TASKS.jsonl) - Task tracking

## Development

This project is developed using the Ralph Wiggum autonomous agent loop. See PROMPT.md for more details.

## License

TBD
