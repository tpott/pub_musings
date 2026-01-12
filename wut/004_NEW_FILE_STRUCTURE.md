# Plan: Modularize job-crawler.ts

## Goal
Break the 795-line `job-crawler.ts` into modular files that:
1. Keep the CLI interface the same
2. Make site extraction debugging intuitive
3. Support adding patterns/extractors for problematic sites

## Current Code Structure (job-crawler.ts)

| Lines | Component | Purpose |
|-------|-----------|---------|
| 10-55 | Types | CliArgs, JobListing, JobBoardPattern, HarEntry, HarLog |
| 57-121 | JOB_BOARD_PATTERNS | Pattern definitions for each ATS |
| 123-186 | createHarRecorder | CDP-based HAR recording |
| 188-199 | createMarkdownConverter | Turndown setup |
| 201-220 | parseArgs | CLI argument parsing |
| 222-332 | Pattern detection | detectJobBoard, detectPatternFromContent, findJobBoardIframeUrl |
| 334-383 | extractJobListings | Find job URLs on careers page |
| 385-511 | Prompting | displayJobPage, promptForSelection |
| 513-533 | File utilities | generateFilename, ensureDirectories |
| 535-669 | crawlJobPage | Crawl individual job pages |
| 671-795 | Main IIFE | Orchestration logic |

## Key Debugging Scenarios

1. **"Job 156" instead of titles** (like SoFi)
   - Issue in `extractJobListings` (line 346-352) - title extraction logic
   - May need site-specific selectors in patterns

2. **Cloudflare Turnstile** (like OpenAI)
   - Bot detection issue, separate from extraction
   - May need browser configuration changes

## Proposed File Structure

```
wut/
├── job-crawler.ts              # Entry point (CLI + main orchestration)
├── lib/
│   ├── types.ts                # All TypeScript interfaces
│   ├── patterns.ts             # Pattern definitions + detection logic
│   ├── extraction.ts           # Job listing extraction (the key debugging file)
│   ├── content.ts              # Job page content extraction + DOM cleaning
│   ├── har-recorder.ts         # HAR recording via CDP
│   ├── iframe.ts               # Job board iframe detection
│   ├── markdown.ts             # HTML to markdown conversion
│   ├── files.ts                # Directory setup, filename generation
│   ├── args.ts                 # CLI argument parsing
│   └── prompt.ts               # Interactive job selection
```

### File Responsibilities

| File | Lines (approx) | Purpose |
|------|----------------|---------|
| `types.ts` | ~60 | All interfaces (CliArgs expanded with flags, JobListing, JobBoardPattern, HarEntry, HarLog) |
| `patterns.ts` | ~120 | JOB_BOARD_PATTERNS + detectJobBoard + detectPatternFromContent |
| `extraction.ts` | ~60 | extractJobListings - **primary debugging target for title issues** |
| `content.ts` | ~80 | DOM cleaning (remove forms/apply sections) + content extraction |
| `har-recorder.ts` | ~70 | createHarRecorder using CDP |
| `iframe.ts` | ~30 | findJobBoardIframeUrl |
| `markdown.ts` | ~20 | createMarkdownConverter (Turndown setup) |
| `files.ts` | ~30 | generateFilename, ensureDirectories |
| `args.ts` | ~25 | parseArgs |
| `prompt.ts` | ~130 | displayJobPage, promptForSelection |
| `job-crawler.ts` | ~150 | Main IIFE + crawlJobPage orchestration |

## Debugging Workflow with New Structure

**When a site extracts wrong titles (e.g., "Job 156"):**
1. Open `lib/extraction.ts` - this is THE file for title extraction
2. Check selectors in `lib/patterns.ts` if the pattern isn't finding links at all
3. The extraction logic is isolated and easy to tweak

**When a site has bot detection issues (e.g., Cloudflare):**
1. Browser setup in `job-crawler.ts` main function
2. Could add stealth/anti-detection options there

## Implementation Steps

1. Create `lib/` directory
2. Create `lib/types.ts` - move all interfaces (CliArgs, JobListing, JobBoardPattern, HarEntry, HarLog)
3. Create `lib/patterns.ts` - move JOB_BOARD_PATTERNS, detectJobBoard, detectPatternFromContent
4. Create `lib/extraction.ts` - move extractJobListings function
5. Create `lib/content.ts` - move DOM cleaning logic + content extraction from crawlJobPage
6. Create `lib/har-recorder.ts` - move createHarRecorder function
7. Create `lib/iframe.ts` - move findJobBoardIframeUrl function
8. Create `lib/markdown.ts` - move createMarkdownConverter function
9. Create `lib/files.ts` - move generateFilename, ensureDirectories functions
10. Create `lib/args.ts` - move parseArgs function, add new flags (--list, --all, --first N, --dry-run)
11. Create `lib/prompt.ts` - move displayJobPage, promptForSelection functions
12. Refactor `job-crawler.ts` - keep main IIFE + crawlJobPage, import everything else

## CLI Compatibility

The basic CLI stays the same:
```bash
bun run job-crawler.ts <careers-url> [pattern]
```

Interactive prompts unchanged:
- `n`/`p` = next/prev page
- `r` = refresh extraction
- `a` = select all
- `d` = done selecting
- `q` = quit
- Number indices for selection (e.g., "1,3,5")

### New: Non-interactive flags for testing

Add optional flags to enable automated/CI testing:

```bash
# List jobs only (no crawling) - useful for testing extraction
bun run job-crawler.ts <url> --list

# Select all and crawl (non-interactive)
bun run job-crawler.ts <url> --all

# Select first N jobs and crawl
bun run job-crawler.ts <url> --first 3

# Dry run - show what would be crawled without actually crawling
bun run job-crawler.ts <url> --all --dry-run
```

This allows Claude to test:
```bash
# Test extraction works (should list job titles, not "Job 1", "Job 2")
bun run job-crawler.ts "https://jobs.lever.co/anthropic" --list

# Test full crawl of first job
bun run job-crawler.ts "https://jobs.lever.co/anthropic" --first 1
```

## Module Exports Summary

Each file will export what's needed:

| File | Exports |
|------|---------|
| `types.ts` | `CliArgs` (with new fields: `list`, `all`, `first`, `dryRun`), `JobListing`, `JobBoardPattern`, `HarEntry`, `HarLog` |
| `patterns.ts` | `JOB_BOARD_PATTERNS`, `detectJobBoard()`, `detectPatternFromContent()` |
| `extraction.ts` | `extractJobListings()` |
| `content.ts` | `cleanDom()`, `extractContent()` |
| `har-recorder.ts` | `createHarRecorder()` |
| `iframe.ts` | `findJobBoardIframeUrl()` |
| `markdown.ts` | `createMarkdownConverter()` |
| `files.ts` | `generateFilename()`, `ensureDirectories()` |
| `args.ts` | `parseArgs()` |
| `prompt.ts` | `displayJobPage()`, `promptForSelection()` |

## Verification

### Automated tests (Claude can run these)

```bash
# 1. Test extraction - should show real job titles, not "Job 1", "Job 2"
bun run job-crawler.ts "https://jobs.lever.co/anthropic" --list

# 2. Test crawl of first job - should produce files in data/
bun run job-crawler.ts "https://jobs.lever.co/anthropic" --first 1

# 3. Verify output files exist
ls -la data/jobs/ data/har/
```

### Manual tests (user runs interactively)

```bash
# Test interactive mode still works
bun run job-crawler.ts "https://jobs.lever.co/anthropic"
```

Verify:
1. Job listings show real titles (not "Job 1", "Job 2", etc.)
2. Pagination works (n/p commands)
3. Refresh works (r command)
4. Job crawling produces HAR + markdown files in `data/`
