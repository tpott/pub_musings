# Media Management Scripts & API

**Status**: Implemented (Tasks 352-359, 360)
**Origin**: User feedback (2026-02-14)

## Goal

Scripts to add new concepts and upload media sets to existing concepts, backed by admin API endpoints. Scripts use `API_SESSION_ID` env var (same pattern as `scripts/fetch-feedback.py`).

## Implementation Summary

All tasks completed:
- **Task 352**: DB methods (`InsertConcept`, `ListConceptsWithCounts`, `NextMediaSetNumber`)
- **Task 353**: Admin concept endpoints (POST + GET /api/admin/concepts)
- **Task 354**: Admin media upload endpoint (POST /api/admin/media)
- **Task 355**: Dynamic `seedMediaFromDisk` (scans all dirs, not hardcoded)
- **Task 356**: `scripts/add-concept.py`
- **Task 357**: `scripts/upload-media.py`
- **Task 358**: Admin user docs in DEPLOY.md
- **Task 359**: API.md updated with admin endpoints
- **Task 360**: CSRF token fetch added to admin scripts (both failed 403 without it)

## Design

### Admin User Docs

Document in DEPLOY.md how to promote a user to admin:

```bash
ssh peekaboo-server
sqlite3 /path/to/data/peekaboo.db "SELECT id, email FROM users;"
# Copy the user ID, then set TRUSTED_USERS in .env
# TRUSTED_USERS=user-id-1,user-id-2
# Restart the service
```

No database schema change needed - `TRUSTED_USERS` is an env var, not a DB column.

### API Endpoints

#### POST /api/admin/concepts

Create a new concept.

**Request:**
```json
{"id": "horse", "name": "Horse"}
```

**Validation:**
- `id`: required, `^[a-z0-9_]+$`, max 50 chars (matches `validConceptPattern`)
- `name`: required, 1-100 chars, trimmed

**Response (201):**
```json
{"id": "horse", "name": "Horse"}
```

**Errors:** 400 (validation), 401 (unauthenticated), 403 (not admin), 409 (concept exists)

#### GET /api/admin/concepts

List all concepts with media set counts.

**Response (200):**
```json
{
  "concepts": [
    {"id": "cat", "name": "Cat", "media_set_count": 1},
    {"id": "horse", "name": "Horse", "media_set_count": 0}
  ]
}
```

#### POST /api/admin/media

Upload a media set for a concept. Multipart form upload.

**Form fields:**
- `concept_id`: required, existing concept ID
- `photo`: required, image file (JPEG, PNG, WebP, GIF)
- `audio`: optional, audio file (MP3, WAV, OGG)
- `video`: optional, video file (MP4, WebM)

**Validation:**
- Photo required for every media set
- Photo: min 320x240, max 10MB
- Audio: max 5MB, max 5 seconds duration
- Video: max 50MB, max 10 seconds duration
- Duration/dimension checks via ffprobe (if available) or file size heuristics

**Storage:**
- Files saved to `data/media/{concept_id}/set{N}/` where N is next available
- Database updated via `SeedMediaSet()`
- If age key is loaded, files are encrypted after saving

**Response (201):**
```json
{
  "concept_id": "horse",
  "set": "set2",
  "photo_path": "data/media/horse/set2/photo.jpg",
  "audio_path": "data/media/horse/set2/audio.mp3"
}
```

### Scripts

#### scripts/add-concept.py

Add a new concept to the database via the admin API.

```bash
python3 scripts/add-concept.py horse "Horse"
python3 scripts/add-concept.py --id sea_turtle --name "Sea Turtle"
```

Uses `PROD_HOST` and `API_SESSION_ID` from env/.env (same as fetch-feedback.py).

#### scripts/upload-media.py

Upload a media set (photo + optional audio/video) to an existing concept.

```bash
python3 scripts/upload-media.py cat photo.jpg audio.mp3
python3 scripts/upload-media.py horse photo.jpg --audio neigh.mp3 --video horse-running.mp4
```

**Metadata printed:**
- File sizes (human-readable)
- Image dimensions (via Pillow or file header parsing)
- Audio duration (if ffprobe available, otherwise file size estimate)
- Video duration and resolution (if ffprobe available)

**Pre-upload validation (client-side):**
- Photo file exists, is image type
- Audio: warn if > 5 seconds (via ffprobe), error if > 5MB
- Video: warn if > 10 seconds (via ffprobe), error if > 50MB
- Print all metadata before uploading, ask for confirmation (--yes to skip)

### Backend Changes

#### Make seedMediaFromDisk dynamic

Currently `seedMediaFromDisk()` hardcodes concept list. Change to scan all subdirectories of `data/media/` and check if matching concepts exist in DB. This way new concepts added via API + file upload just work on restart.

#### DB methods

- `InsertConcept(id, name)`: INSERT into concepts, return error if duplicate
- `GetConceptCount()`: count for each concept (for listing)
- `ListConceptsWithCounts()`: JOIN concepts with COUNT of media_sets
- `NextMediaSetNumber(conceptID)`: find max set number for concept

## Media Constraints

| Type  | Max Size | Max Duration | Min Resolution |
|-------|----------|--------------|----------------|
| Photo | 10 MB    | N/A          | 320x240        |
| Audio | 5 MB     | 5 seconds    | N/A            |
| Video | 50 MB    | 10 seconds   | 320x240        |

## Task Breakdown

1. **Task 352**: Add DB methods (`InsertConcept`, `ListConceptsWithCounts`, `NextMediaSetNumber`) + tests
2. **Task 353**: Add admin concept endpoints (POST + GET /api/admin/concepts) + tests
3. **Task 354**: Add admin media upload endpoint (POST /api/admin/media) + tests
4. **Task 355**: Make `seedMediaFromDisk` dynamic (scan all dirs, not hardcoded list)
5. **Task 356**: Write `scripts/add-concept.py` + test it
6. **Task 357**: Write `scripts/upload-media.py` + test it
7. **Task 358**: Add admin user docs to DEPLOY.md
8. **Task 359**: Update API.md with new admin endpoints
