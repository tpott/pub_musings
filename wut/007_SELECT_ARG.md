# Plan: Add `--select` CLI Argument

## Summary

Add a `--select` CLI argument to allow specifying exact job indices to crawl, e.g., `--select "13,14"`.

## Files to Modify

1. **`lib/types.ts`** - Add `select` field to `CliArgs` interface
2. **`lib/args.ts`** - Parse `--select` argument and add help text
3. **`job-crawler.ts`** - Handle `--select` in the selection logic

## Implementation

### 1. Update `lib/types.ts`

Add a new field to `CliArgs`:

```typescript
export interface CliArgs {
  careersUrl: string;
  pattern?: string;
  list?: boolean;
  all?: boolean;
  first?: number;
  select?: number[];    // NEW: --select "13,14" → [13, 14]
  dryRun?: boolean;
  followIframe?: boolean;
}
```

### 2. Update `lib/args.ts`

Add parsing for `--select`:

```typescript
let select: number[] | undefined;

// In the for loop, add:
} else if (arg === '--select') {
  const nextArg = args[i + 1];
  if (nextArg !== undefined && !nextArg.startsWith('-')) {
    select = nextArg.split(',').map(s => parseInt(s.trim(), 10)).filter(n => !isNaN(n));
    i++; // Skip the value
  }
}
```

Add help text:
```
  --select "N,M,..."  Select specific job indices to crawl (comma-separated)
```

Add example:
```
  bun run job-crawler.ts "https://example.com/careers" --select "1,3,5"
```

Return `select` in the parsed args object.

### 3. Update `job-crawler.ts`

In the selection logic (around line 229), add handling for `--select`:

```typescript
const { careersUrl, pattern: patternName, list, all, first, select, dryRun, followIframe } = parseArgs();

// ... later, in the selection logic:

if (all) {
  selectedIndices = listings.map((job) => job.index);
  console.log(`Selected all ${selectedIndices.length} job(s).`);
} else if (first !== undefined) {
  const count = Math.min(first, listings.length);
  selectedIndices = listings.slice(0, count).map((job) => job.index);
  console.log(`Selected first ${count} job(s).`);
} else if (select !== undefined) {
  // Filter to only indices that exist in listings
  const validIndices = select.filter(idx => listings.some(j => j.index === idx));
  selectedIndices = validIndices;
  if (validIndices.length < select.length) {
    const invalid = select.filter(idx => !validIndices.includes(idx));
    console.log(`Warning: Invalid indices ignored: ${invalid.join(', ')}`);
  }
  console.log(`Selected ${selectedIndices.length} job(s) by index.`);
} else {
  // Interactive mode
  selectedIndices = await promptForSelection(listings, page, pattern);
}
```

## Verification

Run the following command to test:

```bash
bun run job-crawler.ts "https://careers.airbnb.com/positions/?_departments=engineering&_offices=united-states" --no-follow-iframe --select "13,14"
```

Expected behavior:
1. Browser opens and navigates to the Airbnb careers page
2. Job listings are extracted
3. Jobs at indices 13 and 14 are selected (if they exist)
4. Those jobs are crawled and saved to `data/jobs/` and `data/har/`

### Additional test cases

1. **With `--dry-run`** - Verify selected indices without crawling:
   ```bash
   bun run job-crawler.ts "https://careers.airbnb.com/positions/?_departments=engineering&_offices=united-states" --no-follow-iframe --select "13,14" --dry-run
   ```

2. **With `--list` first** - Preview available indices:
   ```bash
   bun run job-crawler.ts "https://careers.airbnb.com/positions/?_departments=engineering&_offices=united-states" --no-follow-iframe --list
   ```

3. **Invalid indices** - Should warn and skip:
   ```bash
   bun run job-crawler.ts "https://careers.airbnb.com/positions/?_departments=engineering&_offices=united-states" --no-follow-iframe --select "999,1000"
   ```
