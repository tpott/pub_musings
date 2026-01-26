# 015 Language Selection for Transcription

## Overview

Add language selection to the upload form so users can specify the language of their audio/video file. This improves transcription accuracy when the language is known.

## Current State

- **Transcription service already supports language**: `TranscribeOptions.Language` field exists and is sent to whisper-server if provided
- **whisper-server accepts language parameter**: The `/inference` endpoint accepts a `language` form field
- **No frontend UI for language**: Users cannot specify language
- **No database storage for language**: Jobs table has no language column
- **Worker doesn't pass language**: Even if stored, worker doesn't read/pass language to transcription

## Implementation Plan

### 1. Database Migration

Create migration `004_add_language_to_jobs.sql`:

```sql
ALTER TABLE jobs ADD COLUMN language TEXT;
```

The language column is nullable - NULL means auto-detect.

### 2. Update Job Model (`internal/db/jobs.go`)

Add `Language` field to Job struct:

```go
type Job struct {
    // ... existing fields ...
    Language     *string   `json:"language,omitempty"`  // ISO 639-1 code, nil = auto-detect
}
```

Update all SQL queries to include language:
- `CreateJob` - INSERT should include language
- `GetJobByID` - SELECT should include language
- `GetJobsByUserID` - SELECT should include language
- `GetOldJobs` - SELECT should include language

### 3. Update Upload Handler (`cmd/server/upload_handlers.go`)

Read language from form and store in job:

```go
// Get language parameter from form (optional, default: auto-detect)
language := r.FormValue("language")
// Validate if provided
if language != "" {
    // Validate it's a supported language code
    validLanguages := map[string]bool{
        "en": true, "es": true, "fr": true, "de": true, "it": true,
        "pt": true, "zh": true, "ja": true, "ko": true, "ru": true,
        // ... more languages ...
    }
    if !validLanguages[language] {
        // reject or warn
    }
}

job := &db.Job{
    // ... existing fields ...
    Language: &language,  // or nil if empty
}
```

### 4. Update Worker (`internal/worker/worker.go`)

Pass language to TranscribeOptions in `processJob`:

```go
opts := transcribe.TranscribeOptions{
    Format: format,
}
if job.Language != nil {
    opts.Language = *job.Language
}

transcript, err := wp.transcribeService.TranscribeFile(job.FilePath, opts)
```

Also update `processEmbeddedJob` similarly.

### 5. Update Frontend (`frontend/src/pages/index.astro`)

Add language dropdown before submit button:

```html
<div class="options-section">
    <label for="language-select">Language (optional)</label>
    <select id="language-select">
        <option value="">Auto-detect</option>
        <option value="en">English</option>
        <option value="es">Spanish</option>
        <option value="fr">French</option>
        <!-- ... more languages ... -->
    </select>
</div>
```

Update FormData submission:

```javascript
const formData = new FormData();
formData.append('file', selectedFile);
const language = document.getElementById('language-select').value;
if (language) {
    formData.append('language', language);
}
```

### 6. Update Dashboard (`frontend/src/pages/dashboard.astro`)

Display language in job cards (optional enhancement).

## Supported Languages

whisper.cpp supports these languages (ISO 639-1 codes):

| Code | Language |
|------|----------|
| en | English |
| zh | Chinese |
| de | German |
| es | Spanish |
| ru | Russian |
| ko | Korean |
| fr | French |
| ja | Japanese |
| pt | Portuguese |
| tr | Turkish |
| pl | Polish |
| ca | Catalan |
| nl | Dutch |
| ar | Arabic |
| sv | Swedish |
| it | Italian |
| id | Indonesian |
| hi | Hindi |
| fi | Finnish |
| vi | Vietnamese |
| he | Hebrew |
| uk | Ukrainian |
| el | Greek |
| ms | Malay |
| cs | Czech |
| ro | Romanian |
| da | Danish |
| hu | Hungarian |
| ta | Tamil |
| no | Norwegian |
| th | Thai |
| ur | Urdu |
| hr | Croatian |
| bg | Bulgarian |
| lt | Lithuanian |
| la | Latin |
| mi | Maori |
| ml | Malayalam |
| cy | Welsh |
| sk | Slovak |
| te | Telugu |
| fa | Persian |
| lv | Latvian |
| bn | Bengali |
| sr | Serbian |
| az | Azerbaijani |
| sl | Slovenian |
| kn | Kannada |
| et | Estonian |
| mk | Macedonian |
| br | Breton |
| eu | Basque |
| is | Icelandic |
| hy | Armenian |
| ne | Nepali |
| mn | Mongolian |
| bs | Bosnian |
| kk | Kazakh |
| sq | Albanian |
| sw | Swahili |
| gl | Galician |
| mr | Marathi |
| pa | Punjabi |
| si | Sinhala |
| km | Khmer |
| sn | Shona |
| yo | Yoruba |
| so | Somali |
| af | Afrikaans |
| oc | Occitan |
| ka | Georgian |
| be | Belarusian |
| tg | Tajik |
| sd | Sindhi |
| gu | Gujarati |
| am | Amharic |
| yi | Yiddish |
| lo | Lao |
| uz | Uzbek |
| fo | Faroese |
| ht | Haitian creole |
| ps | Pashto |
| tk | Turkmen |
| nn | Nynorsk |
| mt | Maltese |
| sa | Sanskrit |
| lb | Luxembourgish |
| my | Myanmar |
| bo | Tibetan |
| tl | Tagalog |
| mg | Malagasy |
| as | Assamese |
| tt | Tatar |
| haw | Hawaiian |
| ln | Lingala |
| ha | Hausa |
| ba | Bashkir |
| jw | Javanese |
| su | Sundanese |

For the initial implementation, we'll include a subset of the most commonly used languages.

## Testing

1. **Unit test**: Test that language is stored and retrieved from database
2. **Upload test**: Verify language is passed from form to job
3. **Worker test**: Verify language is passed to transcription service
4. **E2E test**: Upload with language selection, verify transcription uses it

## Files to Modify

**Backend:**
- `internal/db/migrations/004_add_language_to_jobs.sql` (new)
- `internal/db/jobs.go`
- `cmd/server/upload_handlers.go`
- `internal/worker/worker.go`

**Frontend:**
- `src/pages/index.astro`

**Tests:**
- `internal/db/jobs_test.go` (update/add tests)
- `cmd/server/upload_handlers_test.go` (if exists)

## Verification

**done_when**: "Upload form includes language dropdown; transcription uses selected language model"

1. Start frontend and backend
2. Navigate to upload page
3. Verify language dropdown is visible
4. Select a file and choose a language (e.g., "Spanish")
5. Submit upload
6. Check job in database has language field set
7. Check worker logs show language being passed to transcription
