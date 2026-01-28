# Plan: Add Location Extraction & Filtering to Job Crawler

## Problem

The microsoft.ai careers page has ~600 jobs. URL params like `?selected_regions=redmond-united-states` visually filter to ~80 in the browser, but `r` (refresh extraction) still sees all 600 because `page.$$()` matches hidden DOM elements. The user needs:

1. Location text extracted per job listing
2. Location displayed in `--list` and interactive output
3. A way to filter jobs by location

## Pre-Implementation: Study the Page

Before coding, we should run a quick Puppeteer study script (not Playwright -- Puppeteer is already a dependency) to inspect the microsoft.ai DOM structure and confirm what selectors work for location text. I'll write a small `study-page.ts` script that:
- Navigates to `https://microsoft.ai/careers/`
- Dumps the HTML of a few job listing elements
- Identifies the CSS class/structure used for location text

The user should run: `bun run study-page.ts "https://microsoft.ai/careers/"`

Also try: `bun run job-crawler.ts "https://microsoft.ai/careers/" --list` to see current behavior.

## CLI Design Decision

**Recommendation: Always show location when available. No new display flag.**

- `displayJobPage` in `lib/prompt.ts:16-17` already conditionally displays location (`if (job.location) ...`)
- Once extraction populates the field, it shows automatically
- No location? Nothing prints (graceful degradation)
- Avoids negation flags (`--no-location`) that the user dislikes
- A `--verbose` or `--show-location` flag adds complexity for minimal value

**New flag: `--location "pattern"`** -- Filters extracted listings by case-insensitive substring match on location text. This is a positive/additive filter, not a negation.

## Changes (5 files, ~70 lines)

### 1. `lib/types.ts` -- Add `location` to `CliArgs`

```typescript
location?: string;  // --location "pattern" filter
```

`JobListing.location` already exists, no change needed there.

### 2. `lib/args.ts` -- Parse `--location` flag

Add parsing identical to `--first`/`--select` pattern:
```
} else if (arg === '--location') {
  const nextArg = args[i + 1];
  if (nextArg !== undefined && !nextArg.startsWith('-')) {
    location = nextArg;
    i++;
  }
}
```

Update usage text with the new flag and example.

### 3. `lib/extraction.ts` -- Core change: visibility check + location extraction

**Consolidate the 3 separate `element.evaluate()` calls into 1** (href, title text, parentText are currently 3 round-trips per element -- at 600 elements this is slow). The single consolidated call returns `{ href, text, location } | null`:

- **Visibility check**: Skip elements where `getComputedStyle` shows `display:none` or `visibility:hidden`, or `offsetWidth === 0 && offsetHeight === 0`. Also check the parent container. This fixes the `r` refresh showing 600 instead of 80.

- **Location extraction** (two strategies):
  1. **Selector-based**: Look in the parent container (`el.closest('li, tr, div, article, [class*="card"], [class*="listing"]')`) for a child matching `[class*="location"], [class*="Location"], [class*="region"], [data-field="location"]`
  2. **Heuristic fallback**: Strip the title text from the parent's `textContent`, then regex match a "City, Country/State" pattern: `/([A-Z][a-zA-Z\s]+,\s*[A-Z][a-zA-Z\s]*)/`

### 4. `lib/prompt.ts` -- Pass location filter into `promptForSelection`

Add optional `locationFilter?: string` parameter. On `r` refresh (lines 87-94), apply the filter to re-extracted listings before replacing the list.

### 5. `job-crawler.ts` -- Wire up filtering

- Destructure `location` from `parseArgs()` on line 119
- After each call to `extractJobListings` (line 171 and line 204), filter listings:
  ```typescript
  if (location) {
    listings = listings.filter(
      (job) => job.location?.toLowerCase().includes(location.toLowerCase())
    );
    console.log(`Filtered to ${listings.length} job(s) matching location "${location}".`);
  }
  ```
- Pass `location` to `promptForSelection` on line 252

### 6. `study-page.ts` -- Temporary research script (new file)

Small Puppeteer script to dump the DOM structure of a few job listings from a careers page. This helps confirm selectors before implementation. Delete after use.

## Verification

1. User runs: `bun run study-page.ts "https://microsoft.ai/careers/"` -- confirms DOM structure
2. After implementation, user runs:
   - `bun run job-crawler.ts "https://microsoft.ai/careers/" --list` -- should show location per job
   - `bun run job-crawler.ts "https://microsoft.ai/careers/" --list --location "Redmond"` -- should show only Redmond jobs
   - Interactive mode with `r` refresh on filtered page -- should respect CSS visibility (show ~80, not 600)
