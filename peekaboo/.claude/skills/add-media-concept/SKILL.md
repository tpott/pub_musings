---
name: add-media-concept
description: Use when adding a new animal or concept with media to Peekaboo, or when the user wants to source CC0 photos and audio for a concept
---

# Add Media Concept

Operational runbook for adding a new concept (animal, object, etc.) to Peekaboo with CC0-licensed media. Covers sourcing, uploading, server-side encryption, and verification.

## Prerequisites

- `.env` file in project root with `PROD_HOST` and `API_SESSION_ID`
- Python 3 available locally

## Workflow

### Step 0: Check if concept already exists

```bash
source .env
curl -s -H "Authorization: Bearer $API_SESSION_ID" "$PROD_HOST/api/admin/concepts" | python3 -m json.tool
```

Response lists concepts with media set counts:
```json
{"concepts": [{"id": "cat", "name": "Cat", "media_set_count": 3}]}
```

If the concept already exists, **ask the user** whether to add another media set or stop.

### Step 1: Find a CC0 photo on Wikimedia Commons

Query the Wikimedia Commons API:

```
https://commons.wikimedia.org/w/api.php?action=query&generator=search&gsrsearch=<QUERY>&gsrnamespace=6&prop=imageinfo&iiprop=url|extmetadata&iiurlwidth=640&format=json
```

Use WebFetch to call this API. Filter results by:
- `extmetadata.LicenseShortName.value` must contain `CC0` or `Public domain`
- Prefer real photographs over illustrations or clipart
- Use the `thumburl` from `imageinfo[0]` (640px wide version)

Select the best real photograph with a CC0/public domain license.

### Step 2: Find a CC0 audio clip

Search for a short sound effect (under 5 seconds):

**BigSoundBank** (preferred — all content is CC0):
- Use WebSearch for `site:bigsoundbank.com <animal> sound`
- Browse results with WebFetch to find a suitable clip

**Freesound** (alternative):
- Use WebSearch for `site:freesound.org <animal> sound CC0`
- Filter for CC0 license and short duration

Select the best clip under 5 seconds with a CC0 license.

### Step 3: Download media to /tmp/

```bash
curl -L -o /tmp/photo.jpg "<PHOTO_URL>"
curl -L -o /tmp/audio.mp3 "<AUDIO_URL>"
```

Verify files are valid:
```bash
file /tmp/photo.jpg /tmp/audio.mp3
```

### Step 4: Create the concept

```bash
python3 scripts/add-concept.py <concept_id> "<Display Name>"
```

- `concept_id`: lowercase, underscores for multi-word (e.g., `sea_turtle`)
- Display name: title case (e.g., `Sea Turtle`)
- Exit code 1 means concept already exists (409)

### Step 5: Upload media

```bash
python3 scripts/upload-media.py <concept_id> /tmp/photo.jpg --audio /tmp/audio.mp3 --yes
```

Add `--video /tmp/video.mp4` if a video was also sourced.

**Upload constraints** (enforced by script):
- Photo: max 10 MB, min 320x240
- Audio: max 5 MB, max 5 seconds
- Video: max 50 MB, max 10 seconds

### Step 6: Verify

```bash
source .env
curl -s -H "Authorization: Bearer $API_SESSION_ID" "$PROD_HOST/api/admin/concepts" | python3 -m json.tool
```

Confirm the concept appears with the expected `media_set_count`.

## Common Issues

| Problem | Fix |
|---------|-----|
| 403 on API calls | `API_SESSION_ID` expired — get a new session token |
| add-concept.py exits 1 | Concept exists — skip to step 5 to add media set |
| Upload validation fails | Check file size/dimensions against constraints above |
| `age` not found on server | Install: `sudo apt install age` |
| No CC0 results on Wikimedia | Broaden search terms; try synonyms or related species |
