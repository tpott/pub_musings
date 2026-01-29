# Video Thumbnails

## Overview

Video thumbnails provide visual previews in the video list. A JPEG thumbnail is extracted from each uploaded video using ffmpeg, encrypted at rest alongside the video, and served via a dedicated API endpoint with caching.

## Status: Implemented

Created: 2026-01-28

## How Thumbnails Are Generated

### ffmpeg Frame Extraction

`backend/audio/audio.go` — `GenerateThumbnail(videoPath, thumbnailPath string) error`

1. Determine video duration via ffprobe (`GetVideoDuration`)
2. Seek to 10% of duration (minimum 1 second, fallback 3 seconds if duration unavailable)
3. Extract a single frame as JPEG

```bash
ffmpeg -ss <seekTime> -i <videoPath> \
  -vframes 1 \
  -q:v 2 \
  -vf "scale=320:180:force_original_aspect_ratio=decrease" \
  -y <thumbnailPath>
```

| Parameter | Value | Purpose |
|-----------|-------|---------|
| `-ss` | 10% of duration | Seek position (before input for fast seeking) |
| `-vframes` | 1 | Extract exactly one frame |
| `-q:v` | 2 | JPEG quality (1-31 scale, lower is better) |
| `-vf scale` | 320:180 | Max dimensions, preserves aspect ratio |
| `-y` | — | Overwrite existing file |

### When Thumbnails Are Created

Thumbnails are generated during the upload flow, after video validation but before database insertion:

- **Single-file upload** (`main.go`, upload handler): generates thumbnail from validated video, then encrypts it
- **Chunked upload completion** (`main.go`, chunked completion handler): same logic after chunks are reassembled

Thumbnail generation is **non-fatal** — if ffmpeg fails, the upload proceeds without a thumbnail. A warning is logged and the frontend shows a placeholder.

## Storage

### On Disk

- **Temporary path**: `{uploadDir}/{uploadID}_thumb.jpg` (unencrypted, deleted after encryption)
- **Encrypted path**: `{uploadDir}/{uploadID}_thumb.jpg.age` (age encryption, persisted)
- **Key version**: Same key version as the video file

### In Database

Migration `002_add_thumbnail_path.up.sql`:

```sql
ALTER TABLE videos ADD COLUMN thumbnail_path TEXT;
```

The `thumbnail_path` column stores the path to the encrypted `.age` file. It is nullable — `NULL` when no thumbnail was generated.

## API Endpoint

### GET /api/videos/{id}/thumbnail

Serves the decrypted thumbnail image.

**Flow:**

1. Parse video ID from URL path
2. Fetch video record from database
3. Check `thumbnail_path` is non-null and non-empty (404 if missing)
4. Validate path is within the upload directory (403 if path traversal detected)
5. Verify file exists on disk (404 if missing)
6. Generate ETag: `sha256("thumb-{videoID}-{createdAtUnix}")`, truncated to 16 hex chars
7. Check `If-None-Match` header — return 304 if ETag matches
8. Set cache headers (24-hour private cache)
9. Decrypt `.age` file to a temporary file
10. Serve via `http.ServeFile`, then delete the temporary file

**Response Headers:**

| Header | Value |
|--------|-------|
| `Content-Type` | `image/jpeg` (set by `http.ServeFile`) |
| `Cache-Control` | `private, max-age=86400` |
| `ETag` | `"<16-hex-char-hash>"` |

**Status Codes:**

| Code | Condition |
|------|-----------|
| 200 | Thumbnail served |
| 304 | ETag matches `If-None-Match` |
| 403 | Path traversal attempt |
| 404 | Video not found, no thumbnail, or file missing on disk |
| 500 | Decryption failure |

## Frontend Display

### Video List (`videos.astro`)

Each video card shows either a thumbnail image or a placeholder:

```html
<!-- Thumbnail exists -->
<img class="video-thumbnail"
     src="/api/videos/{id}/thumbnail"
     alt="Thumbnail for {filename}"
     loading="lazy" />

<!-- No thumbnail -->
<div class="video-thumbnail-placeholder">&#x1f3ac;</div>
```

**Styling:**

| Property | Desktop | Mobile |
|----------|---------|--------|
| Width | 80px | 100% |
| Height | 45px | auto (16:9 aspect ratio) |
| Border radius | 4px | 4px |

When the video has a completed transcription, the thumbnail is clickable (opens the video player modal). Keyboard accessible via `role="button"` and `tabindex="0"`.

### API Schema (`api-schemas.ts`)

```typescript
export const VideoSchema = z.object({
    // ... other fields ...
    thumbnail_path: z.string().optional(),
});
```

The frontend checks `video.thumbnail_path && video.thumbnail_path !== ''` to decide whether to render an `<img>` or the placeholder `<div>`.

## Caching Strategy

Thumbnails are immutable — generated once at upload time and never modified. This makes aggressive caching safe.

- **ETag**: Derived from video ID + creation timestamp. Deterministic and stable.
- **Cache-Control**: `private, max-age=86400` (24 hours). `private` because thumbnails are user-specific content behind authentication.
- **Conditional requests**: Clients send `If-None-Match` on subsequent requests. Server returns 304 Not Modified if the ETag matches, avoiding re-decryption and re-transfer.

## Lifecycle

1. **Upload**: Thumbnail generated from video at 10% duration mark
2. **Encryption**: Thumbnail encrypted with current key version, unencrypted copy deleted
3. **Database**: Encrypted path saved to `videos.thumbnail_path`
4. **Serving**: Decrypted on-demand to temp file, served, temp file deleted
5. **Deletion**: When video is deleted, thumbnail file is removed from disk along with the video file

## Security

- **Encryption at rest**: Thumbnails are encrypted using age, same key version as the video
- **Path traversal protection**: `pathValidator.ValidateAbsolutePath()` ensures thumbnail path is within the upload directory before serving
- **Temporary decryption**: Decrypted file is created with `os.CreateTemp` and removed via `defer os.Remove` after serving
- **Authentication**: Endpoint requires valid session (same auth as other video endpoints)

## Testing

### Backend Unit Tests (`audio_test.go`)

- `TestGenerateThumbnail_NonexistentFile` — error handling for missing input
- `TestGenerateThumbnail_FFmpegNotAvailable` — graceful handling when ffmpeg not in PATH

### Backend API Tests (`api_test.go`)

- `TestThumbnailEndpointSuccess` — 200 response with correct content type, cache headers, and ETag
- `TestThumbnailEndpointNotFound` — 404 for nonexistent video
- `TestThumbnailEndpointNoThumbnail` — 404 for video without thumbnail
- `TestThumbnailEndpointConditionalRequest` — 304 when If-None-Match matches
- Path traversal test — 403 when thumbnail_path contains `/etc/shadow`
