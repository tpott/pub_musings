# Plan: Fix job-crawler.ts Bugs

## File to Modify
`/Users/trevor/Github/pub_musings/wut/job-crawler.ts`

---

## Bug 1: "No job listings found" should allow navigation instead of exiting

### Current Behavior (lines 637-641)
When no job listings are found initially, the program prints a message and calls `return;`, which exits and closes the browser immediately.

### Desired Behavior
Should behave like the refresh case (lines 394-396) - allow the user to navigate around in the browser and type "refresh" to retry extraction.

### Implementation
1. Replace the early `return` at lines 637-641 with a loop that:
   - Displays the "No job listings found" message with pattern suggestions
   - Prompts user for input: "r" to refresh, "q" to quit
   - On "r": re-extract listings and break out of loop if found
   - On "q": exit gracefully
   - Loop continues until listings found or user quits

---

## Bug 2: Log when page content matches a known pattern but using generic

### Current Behavior
Pattern detection is URL-based only (`detectJobBoard` at lines 223-231). When a URL defaults to "generic", there's no inspection of page content to detect known patterns.

### Desired Behavior
After extracting with the generic pattern, analyze the page content to detect if it appears to match a known pattern (ashby, greenhouse, lever, workday). If detected, log a suggested re-run command.

### Implementation
1. Add new function `detectPatternFromContent(page: Page): Promise<string | null>` that:
   - Checks for pattern-specific selectors/URLs in the page
   - For each known pattern, look for:
     - URLs containing pattern identifiers (e.g., `ashbyhq.com`, `greenhouse.io`)
     - Pattern-specific CSS classes/attributes
   - Returns the pattern name if detected, null otherwise

2. Call this function after initial extraction when using "generic" pattern (after line 635):
   - If a different pattern is detected, log:
     ```
     Note: Page content appears to match the '{pattern}' pattern.
     Re-run with: bun run job-crawler.ts "{url}" {pattern}
     ```

---

## Bug 3: Handle iframes containing job details

### Current Behavior
Iframe detection (`findJobBoardIframeUrl` at lines 233-259) only runs on the initial careers page load (lines 625-632). When crawling individual job pages in `crawlJobPage`, iframes are not checked.

### Example
URL: `https://www.aurelian.com/careers?ashby_jid=954430b4-ddc1-41df-b4b2-29f1cffc7c5a`
The job details are loaded in an ashby iframe, not in the main page content.

### Implementation
1. Modify `crawlJobPage` function (starting at line 463) to:
   - After page navigation (line 476), check for job board iframes using `findJobBoardIframeUrl(page)`
   - If iframe found:
     - Log detection message: "Detected job board iframe, navigating to: {iframeUrl}"
     - Navigate to the iframe URL directly (consistent with careers page behavior)
     - Re-detect pattern for the iframe URL
   - Continue with content extraction from the (potentially new) page

---

## Implementation Order
1. Bug 1 (no listings loop) - self-contained change
2. Bug 2 (pattern detection logging) - adds new function + call site
3. Bug 3 (iframe in job details) - modifies crawlJobPage function
