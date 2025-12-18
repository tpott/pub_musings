# Job Listing Crawler Implementation Plan

## Overview

Create a TypeScript job crawler that:
1. Crawls a careers page and extracts job listing URLs
2. Prompts user to select which jobs to crawl (simple text input)
3. Records HAR files for network requests
4. Converts job pages to markdown files

## Files to Create/Modify

### New Files
- `wut/job-crawler.ts` - Main crawler script (~200 lines)

### Files to Modify
- `wut/package.json` - Add `turndown` dependency and `@types/turndown`
- `wut/.gitignore` - Add `data/` directory

## Dependencies to Add

```json
{
  "dependencies": {
    "turndown": "^7.2.0"
  },
  "devDependencies": {
    "@types/turndown": "^5.0.5"
  }
}
```

## Implementation Steps

### Step 1: Update package.json and .gitignore
- Add turndown for HTML-to-markdown conversion
- Add `data/` to .gitignore

### Step 2: Create job-crawler.ts

Structure (single file for simplicity):

```
1. Imports (puppeteer, turndown, fs, path, readline)
2. Type definitions (CliArgs, JobListing, JobBoardPattern)
3. Job board patterns (greenhouse, lever, workday, ashby, generic)
4. HAR recorder using CDP (createHarRecorder function)
5. HTML-to-markdown converter (createMarkdownConverter function)
6. Helper functions:
   - parseArgs() - CLI argument parsing
   - detectJobBoard() - auto-detect pattern from URL
   - extractJobListings() - find job URLs on careers page
   - promptForSelection() - text-based job selection
   - crawlJobPage() - navigate, record HAR, save markdown
7. Main IIFE
```

## CLI Usage

```bash
bun run job-crawler.ts <careers-url> [pattern]

# Examples:
bun run job-crawler.ts "https://example.com/careers#open-positions"
bun run job-crawler.ts "https://example.com/careers" lever
```

## Job Board Patterns

Support auto-detection and explicit selection:
- `greenhouse` - job-boards.greenhouse.io URLs
- `lever` - jobs.lever.co URLs
- `workday` - myworkdayjobs.com URLs
- `ashby` - jobs.ashbyhq.com URLs
- `generic` - fallback with common job URL patterns

Each pattern defines:
- `name` - Display name
- `urlPattern` - Regex to match job listing URLs
- `selectors` - CSS selectors to find job links on careers page
- `contentSelector` - CSS selector for job content on detail page

## HAR Recording

Use Puppeteer CDP directly (no extra dependency):

```typescript
const client = await page.createCDPSession();
await client.send('Network.enable');

// Listen to events:
// - Network.requestWillBeSent -> capture request details
// - Network.responseReceived -> capture response metadata
// - Network.loadingFinished -> retrieve response body

// Build HAR 1.2 format entries
```

## Output Structure

```
wut/data/
├── har/
│   └── backend-engineer-2024-12-17T10-30-00.har
└── jobs/
    └── backend-engineer-2024-12-17T10-30-00.md
```

## Markdown Output Format

```markdown
# Backend Engineer

**Source:** https://job-boards.greenhouse.io/...
**Fetched:** 2024-12-17T10:30:00.000Z

---

[converted job description content]
```

## Interactive Flow

1. Launch browser (headless: false, reuse chrome-user-data/)
2. Navigate to careers URL
3. Extract job listings using pattern selectors
4. Print numbered list of jobs found
5. Prompt: `Enter indices (e.g., "1,3,5" or "all" or "q"):`
6. For each selected job:
   - Start HAR recording
   - Navigate to job page
   - Stop HAR recording and save to `data/har/`
   - Extract HTML content and convert to markdown
   - Save to `data/jobs/`
7. Close browser

## Key Patterns (from schwab-login.ts)

- Type-only imports: `import { type Browser, type Page } from 'puppeteer'`
- ES modules __dirname: `const __dirname = dirname(fileURLToPath(import.meta.url))`
- Explicit null checks: `if (element === null)`
- Persistent browser session via `chrome-user-data/`
- `waitUntil: 'networkidle2'` for page loads

## Type Definitions

```typescript
interface CliArgs {
  careersUrl: string;
  pattern?: string;
}

interface JobListing {
  index: number;
  title: string;
  url: string;
  department?: string;
  location?: string;
}

interface JobBoardPattern {
  name: string;
  urlPattern: RegExp;
  selectors: string[];
  contentSelector: string;
}

interface HarEntry {
  startedDateTime: string;
  time: number;
  request: {
    method: string;
    url: string;
    headers: Array<{ name: string; value: string }>;
  };
  response: {
    status: number;
    statusText: string;
    headers: Array<{ name: string; value: string }>;
    content: {
      size: number;
      mimeType: string;
      text?: string;
    };
  };
}
```

## Error Handling

- Explicit null checks (following schwab-login.ts patterns)
- Try-catch around individual job crawls (continue on failure)
- Graceful degradation if HAR body capture fails
- Clear console messages at each step
- Browser cleanup in finally block
