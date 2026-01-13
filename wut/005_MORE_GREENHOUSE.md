# Plan: Add greenhouse-embed Pattern for SoFi-like Sites

## Problem

SoFi's careers page uses Greenhouse as backend but has a custom frontend:
- URLs: `sofi.com/careers/job/[ID]?gh_jid=[ID]` (not `greenhouse.io`)
- No `.opening` class or `data-mapped` attributes
- Jobs rendered client-side from embedded JSON
- **Job URLs are in `data-link` attribute, not `href`**
- **Job titles are in `.job-title` span elements**

SoFi's actual HTML structure:
```html
<div class="listing">
  <a data-link="https://sofi.com/careers/job/123?gh_jid=456">
    <span class="job-title">Senior Analyst</span>
    <span class="job-location">San Francisco, CA</span>
  </a>
</div>
```

Current extractors fail because:
- **Generic**: `a[href*="/job"]` matches too many non-job links
- **Greenhouse**: Looks for `greenhouse.io` URLs which SoFi doesn't use
- **Extraction logic**: Only reads `href` attribute, not `data-link`

## Solution

1. Add a `greenhouse-embed` pattern to `lib/patterns.ts` with selectors targeting `data-link` attributes
2. Update `lib/extraction.ts` to fall back to `data-link` when `href` is empty

## Implementation

### File 1: `lib/patterns.ts`

Add new pattern after the `greenhouse` entry (around line 15):

```typescript
'greenhouse-embed': {
  name: 'Greenhouse Embed',
  urlPattern: /gh_jid=/,  // Detect pages with Greenhouse job IDs
  selectors: [
    'a[data-link*="gh_jid="]',     // SoFi-style: data-link attribute
    '.listing a[data-link]',       // Jobs in .listing containers
    'a[href*="gh_jid="]',          // Standard href with gh_jid
    'a[href*="/careers/job/"]',    // Common careers path pattern
  ],
  contentSelector: 'main, article, .content, #content, .job-description',
},
```

Also update `detectPatternFromContent()` to detect `gh_jid` in links (around line 95):

```typescript
{
  name: 'greenhouse-embed',
  urlPatterns: [/gh_jid=/, /\/careers\/job\//],
  classPatterns: [],
},
```

### File 2: `lib/extraction.ts`

Update `extractJobListings()` to check `data-link` when `href` is empty.

Change the href extraction (around line 15) from:
```typescript
const href = await element.evaluate((el) => el.getAttribute('href'));
```

To:
```typescript
const href = await element.evaluate((el) =>
  el.getAttribute('href') || el.getAttribute('data-link')
);
```

Update the title extraction to check for `.job-title` class first:
```typescript
const text = await element.evaluate((el) => {
  // Check for job-title class first (SoFi pattern)
  const jobTitle = el.querySelector('.job-title');
  if (jobTitle) return jobTitle.textContent?.trim() || '';

  // Fall back to heading tags
  const heading = el.querySelector('h1, h2, h3, h4, h5, h6');
  if (heading) return heading.textContent?.trim() || '';

  return el.innerText?.trim() || '';
});
```

## Usage

```bash
bun run job-crawler.ts "https://www.sofi.com/careers/" greenhouse-embed --list
```

## Verification

1. Run the command above
2. Verify job listings appear with proper titles (not "Job 1", "Job 2")
3. Confirm job count is reasonable (SoFi has ~226 jobs)
4. Check that URLs contain `gh_jid=` parameter
