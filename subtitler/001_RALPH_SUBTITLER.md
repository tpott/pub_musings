# 001_RALPH_SUBTITLER

## Overview

Build a web-based subtitling service that allows users to upload video/audio files and receive transcribed subtitles or files with embedded subtitles. The service will use whisper.cpp for transcription and be deployed using the same infrastructure pattern as the personal site.

## Key Decisions

| Decision | Choice |
|----------|--------|
| Core Product | Web UI for video/audio upload → subtitles/embedded files |
| Revenue Model | Free initially (validate market), then pay-per-use |
| Target Audience | Content creators (initially) |
| Processing | Server-side whisper.cpp |
| Frontend | Astro |
| Backend | Go (fork/extend whisper.cpp server pattern) |
| Auth | Email + Password (initial), magic links and OAuth later |
| File Storage | Local initially, encrypted with age (separate key from env vars) |
| File Size Limit | ~10 minutes of 1080p MP4 video |
| Database | SQLite (can migrate later) |
| Job Processing | Background queue (notify on completion) |
| Whisper Model | medium (default) |
| Email Provider | Resend API (see personal/ for pattern) |
| Payments | Stripe (after market validation) |
| Domain | TBD |

## Architecture

```
User Browser ──→ Cloudflare Tunnel ──→ Caddy (static assets)
                                   └──→ Go API Server
                                          ├── /api/auth/* (email+password)
                                          ├── /api/upload (multipart files)
                                          ├── /api/jobs/* (job status)
                                          └── SQLite (users, jobs)

Go Worker (background) ──→ Job Queue (SQLite) ──→ whisper.cpp (medium model)
                                              └──→ ffmpeg (embed subtitles)
```

## High-Level Tasks (for TASKS.jsonl)

### Phase 1: Foundation
1. **Initialize project structure** - Create Astro frontend + Go backend scaffolding
2. **Implement whisper.cpp integration** - Fork/adapt server.cpp for our use case
3. **Basic file upload UI** - Single-page upload form, no auth required
4. **Basic transcription endpoint** - POST audio → return SRT

### Phase 2: User System
5. **Database setup** - SQLite for users/jobs, file storage strategy
6. **Email+password auth** - Registration, login, sessions
7. **User dashboard** - View past jobs, download files

### Phase 3: Features
8. **Background job queue** - Worker processes jobs, stores results, notifies user
9. **Multiple output formats** - SRT, VTT, embedded video (ffmpeg)
10. **Job status & notifications** - Polling/websocket for progress, email on completion

### Phase 4: Deployment
11. **Cloudflare Tunnel setup** - Expose both frontend and API
12. **CI/CD pipeline** - GitHub webhook deployment (similar to personal site)
13. **Secrets management** - sops+age for API keys, DB credentials

### Phase 5: Validation & Growth
14. **MARKETING_PLAN.md** - Define content creator outreach strategy
15. **EXPERIMENTATION_PLAN.md** - A/B testing, audience discovery framework
16. **Analytics integration** - Track usage patterns for market validation

## Files to Create

```
subtitler/
├── TASKS.jsonl              # Ralph Wiggum task management
├── CURRENT_TASK.md          # Active task tracker
├── README.md                # Project overview
├── INSTALL.md               # Setup instructions
├── MARKETING_PLAN.md        # Outreach strategy
├── EXPERIMENTATION_PLAN.md  # Audience testing framework
├── frontend/                # Astro app
│   ├── src/
│   └── package.json
├── backend/                 # Go server
│   ├── cmd/server/
│   ├── internal/
│   │   ├── auth/
│   │   ├── transcribe/
│   │   └── storage/
│   └── go.mod
└── deploy/                  # Deployment configs
    ├── caddy/
    └── cloudflare/
```

## Critical Dependencies

- **whisper.cpp** - Already at ~/Github/whisper.cpp - will reference or fork server.cpp
- **ffmpeg** - For audio conversion and subtitle embedding
- **Cloudflare account** - For tunnel setup
- **Domain** - TBD, needed for production deployment

## Verification Plan

1. **Local dev**: Run `go run ./cmd/server` + `npm run dev` in frontend
2. **Upload test**: Upload a short audio file, verify SRT output
3. **Auth test**: Register, login, verify session persists
4. **E2E test**: Upload → process → download embedded video
5. **Deploy test**: Push to trunk, verify auto-deploy via webhook

## Future Direction

These features are planned but not yet in TASKS.jsonl:

1. **Magic link authentication** - After email+password is stable, add passwordless login via email magic links. Requires reliable email delivery (Resend API is ready).

2. **OAuth integration** - Add Google and GitHub OAuth for better UX. Requires setting up API keys with OAuth providers.

3. **Stripe payment integration** - Once market is validated with free tier, add pay-per-use billing via Stripe checkout.

4. **S3-compatible storage** - If local storage becomes a bottleneck, migrate to S3-compatible storage (Cloudflare R2, AWS S3, etc.).
