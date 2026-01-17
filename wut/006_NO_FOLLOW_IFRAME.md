# Plan: Add `--follow-iframe` / `--no-follow-iframe` CLI flags

## Problem Analysis

The job-crawler has iframe detection/following logic in **two places**, but they behave inconsistently:

### Location 1: Initial careers page (`job-crawler.ts:155-162`)
```typescript
// Check for job board iframe and redirect if found
const iframeUrl = await findJobBoardIframeUrl(page);
if (iframeUrl !== null) {
  console.log(`\nDetected job board iframe, redirecting to: ${iframeUrl}`);
  await page.goto(iframeUrl, { waitUntil: 'networkidle2', timeout: 60000 });
  pattern = detectJobBoard(iframeUrl);  // <-- overwrites user's pattern!
  console.log(`Using pattern for iframe: ${pattern.name}`);
}
```
**Bug**: This code ALWAYS follows iframes, ignoring the user-specified pattern.

### Location 2: Individual job detail pages (`job-crawler.ts:42-55`)
```typescript
// Skip iframe navigation for embedded patterns (content is on parent page, not iframe)
if (pattern.name !== 'Greenhouse Embed') {
  const iframeUrl = await findJobBoardIframeUrl(page);
  ...
}
```
**Correct behavior**: This code skips iframe following for 'Greenhouse Embed' pattern.

### Why Airbnb fails

1. **Without `greenhouse-embed`**: Iframe is followed → jobs extracted → but individual job pages return empty content (Greenhouse pattern selectors don't match)

2. **With `greenhouse-embed`**: Iframe is STILL followed at Location 1 → pattern gets overwritten to `Greenhouse` → `greenhouse-embed` selectors never used → job listings fail

## Proposed Solution

Add explicit `--follow-iframe` / `--no-follow-iframe` CLI flags:

| Flag | Behavior |
|------|----------|
| `--follow-iframe` | Follow detected job board iframes (navigate into them) |
| `--no-follow-iframe` | Stay on parent page, don't follow iframes |
| Neither (default) | Keep current behavior for backward compatibility |

## Implementation Steps

### 1. Update `lib/types.ts`
Add `followIframe?: boolean` to `CliArgs` interface.

### 2. Update `lib/args.ts`
Parse the new flags:
- `--follow-iframe` → `followIframe: true`
- `--no-follow-iframe` → `followIframe: false`
- Neither → `followIframe: undefined` (use default logic)

Update usage/help text.

### 3. Update `job-crawler.ts`

**Location 1 (line 155-162)** - Initial careers page:
```typescript
// Determine if we should follow iframes
const shouldFollowIframe = followIframe ?? (pattern.name !== 'Greenhouse Embed');

if (shouldFollowIframe) {
  const iframeUrl = await findJobBoardIframeUrl(page);
  if (iframeUrl !== null) {
    // existing iframe follow logic...
  }
}
```

**Location 2 (line 42-55)** - Job detail pages:
```typescript
// Check for job board iframe in job detail page
// Skip iframe navigation for embedded patterns OR if explicitly disabled
const shouldFollowIframe = followIframe ?? (pattern.name !== 'Greenhouse Embed');

if (shouldFollowIframe) {
  const iframeUrl = await findJobBoardIframeUrl(page);
  // existing logic...
}
```

Note: Will need to pass `followIframe` to `crawlJobPage()` function.

### 4. Files to modify

| File | Changes |
|------|---------|
| `wut/lib/types.ts:11` | Add `followIframe?: boolean` to `CliArgs` |
| `wut/lib/args.ts:12,28-30,35-42` | Parse `--follow-iframe`/`--no-follow-iframe`, update help |
| `wut/job-crawler.ts:117,155-162` | Destructure new arg, guard Location 1 |
| `wut/job-crawler.ts:23-27,42-55` | Pass to `crawlJobPage()`, guard Location 2 |

## Verification

After implementation, test with:

```bash
# Should stay on parent page and use greenhouse-embed selectors
bun run job-crawler.ts "https://careers.airbnb.com/positions/?_departments=engineering&_offices=united-states" greenhouse-embed --no-follow-iframe --list

# Compare to current behavior (following iframe)
bun run job-crawler.ts "https://careers.airbnb.com/positions/?_departments=engineering&_offices=united-states" greenhouse-embed --follow-iframe --list
```

Expected: `--no-follow-iframe` with `greenhouse-embed` pattern should extract job listings from the parent page without navigating to the Greenhouse iframe.
