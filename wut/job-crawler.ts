import puppeteer, { type Browser, type Page } from 'puppeteer';
import type TurndownService from 'turndown';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';
import { createInterface } from 'readline';
import { existsSync, mkdirSync, writeFileSync } from 'fs';

// Import from lib modules
import type { JobBoardPattern, JobListing } from './lib/types.js';
import { JOB_BOARD_PATTERNS, detectJobBoard, detectPatternFromContent } from './lib/patterns.js';
import { extractJobListings } from './lib/extraction.js';
import { cleanDom, extractContent } from './lib/content.js';
import { createHarRecorder } from './lib/har-recorder.js';
import { findJobBoardIframeUrl } from './lib/iframe.js';
import { createMarkdownConverter } from './lib/markdown.js';
import { generateFilename, ensureDirectories } from './lib/files.js';
import { parseArgs } from './lib/args.js';
import { displayJobPage, promptForSelection } from './lib/prompt.js';

const __dirname = dirname(fileURLToPath(import.meta.url));

// Crawl a single job page
async function crawlJobPage(
  page: Page,
  job: JobListing,
  pattern: JobBoardPattern,
  turndown: TurndownService,
  followIframe?: boolean
): Promise<void> {
  console.log(`\nCrawling: ${job.title}`);
  console.log(`URL: ${job.url}`);

  const harRecorder = await createHarRecorder(page);
  await harRecorder.start();

  try {
    await page.goto(job.url, { waitUntil: 'networkidle2', timeout: 30000 });
  } catch (error) {
    console.log(`  Warning: Page load timed out, continuing anyway...`);
  }

  // Check for job board iframe in job detail page
  // Skip iframe navigation if explicitly disabled or using embedded pattern
  const shouldFollowIframe = followIframe ?? (pattern.name !== 'Greenhouse Embed');
  if (shouldFollowIframe) {
    const iframeUrl = await findJobBoardIframeUrl(page);
    if (iframeUrl !== null) {
      console.log(`  Detected job board iframe, navigating to: ${iframeUrl}`);
      try {
        await page.goto(iframeUrl, { waitUntil: 'networkidle2', timeout: 30000 });
      } catch (error) {
        console.log(`  Warning: Iframe page load timed out, continuing anyway...`);
      }
      pattern = detectJobBoard(iframeUrl);
      console.log(`  Using pattern for iframe: ${pattern.name}`);
    }
  }

  // Wait for client-side rendered content to load
  const contentSelectors = pattern.contentSelector.split(',').map((s) => s.trim());
  try {
    await page.waitForFunction(
      (selectors: string[]) => {
        for (const selector of selectors) {
          const el = document.querySelector(selector);
          if (el && el.textContent && el.textContent.trim().length > 100) {
            return true;
          }
        }
        return false;
      },
      { timeout: 10000 },
      contentSelectors
    );
    console.log(`  Content loaded.`);
  } catch {
    console.log(`  Warning: Timed out waiting for content, continuing anyway...`);
  }

  const harLog = await harRecorder.stop();

  // Save HAR file
  const filename = generateFilename(job.title);
  const domain = new URL(job.url).hostname;
  const harDir = join(__dirname, 'data', 'har', domain);
  if (!existsSync(harDir)) mkdirSync(harDir, { recursive: true });
  const harPath = join(harDir, `${filename}.har`);
  writeFileSync(harPath, JSON.stringify({ log: harLog }, null, 2));
  console.log(`  HAR saved: ${harPath}`);

  // Clean DOM before extracting content
  await cleanDom(page);

  // Extract content and convert to markdown
  const html = await extractContent(page, pattern);
  const markdown = turndown.turndown(html);

  // Build markdown output
  const output = [
    `# ${job.title}`,
    '',
    `**Source:** ${job.url}`,
    `**Fetched:** ${new Date().toISOString()}`,
    '',
    '---',
    '',
    markdown,
  ].join('\n');

  const jobsDir = join(__dirname, 'data', 'jobs', domain);
  if (!existsSync(jobsDir)) mkdirSync(jobsDir, { recursive: true });
  const jobsPath = join(jobsDir, `${filename}.md`);
  writeFileSync(jobsPath, output);
  console.log(`  Markdown saved: ${jobsPath}`);
}

// Main IIFE
(async (): Promise<void> => {
  const { careersUrl, pattern: patternName, list, all, first, select, dryRun, followIframe, location } = parseArgs();

  // Determine which pattern to use
  let pattern: JobBoardPattern;
  if (patternName !== undefined && JOB_BOARD_PATTERNS[patternName] !== undefined) {
    pattern = JOB_BOARD_PATTERNS[patternName];
    console.log(`Using pattern: ${pattern.name}`);
  } else {
    pattern = detectJobBoard(careersUrl);
    console.log(`Auto-detected pattern: ${pattern.name}`);
  }

  // Ensure output directories exist
  ensureDirectories(__dirname);

  // Set up persistent browser session
  const userDataDir = join(__dirname, 'chrome-user-data');

  console.log('Launching browser...');
  const browser: Browser = await puppeteer.launch({
    headless: false,
    userDataDir: userDataDir,
    args: [
      `--user-data-dir=${userDataDir}`,
      '--no-sandbox',
      '--disable-setuid-sandbox',
    ],
  });

  let page: Page | null = null;

  try {
    page = await browser.newPage();
    await page.setViewport({ width: 1280, height: 800 });

    console.log(`\nNavigating to: ${careersUrl}`);
    await page.goto(careersUrl, { waitUntil: 'networkidle2', timeout: 60000 });

    // Check for job board iframe and redirect if found
    // Skip iframe navigation if explicitly disabled or using embedded pattern
    const shouldFollowIframe = followIframe ?? (pattern.name !== 'Greenhouse Embed');
    if (shouldFollowIframe) {
      const iframeUrl = await findJobBoardIframeUrl(page);
      if (iframeUrl !== null) {
        console.log(`\nDetected job board iframe, redirecting to: ${iframeUrl}`);
        await page.goto(iframeUrl, { waitUntil: 'networkidle2', timeout: 60000 });
        pattern = detectJobBoard(iframeUrl);
        console.log(`Using pattern for iframe: ${pattern.name}`);
      }
    }

    console.log('Extracting job listings...');
    let listings = await extractJobListings(page, pattern);

    // Apply location filter if provided
    if (location) {
      const lowerFilter = location.toLowerCase();
      listings = listings.filter(
        (job) => job.location?.toLowerCase().includes(lowerFilter)
      );
      // Re-index after filtering
      listings.forEach((job, i) => { job.index = i + 1; });
      console.log(`Filtered to ${listings.length} job(s) matching location "${location}".`);
    }

    // If no listings found, allow user to navigate and retry (unless in non-interactive mode)
    while (listings.length === 0) {
      console.log('\nNo job listings found. Try a different pattern or check the URL.');
      console.log('Available patterns: greenhouse, lever, workday, ashby, generic');

      // In non-interactive mode, exit
      if (list || all || first !== undefined || select !== undefined) {
        console.log('\nExiting (non-interactive mode).');
        return;
      }

      console.log('\nYou can navigate in the browser, scroll, or adjust filters.');

      const rl = createInterface({
        input: process.stdin,
        output: process.stdout,
      });

      const answer = await new Promise<string>((resolve) => {
        rl.question('Enter "r" to refresh/retry extraction, "q" to quit: ', resolve);
      });
      rl.close();

      const trimmed = answer.toLowerCase().trim();
      if (trimmed === 'q' || trimmed === 'quit') {
        console.log('Exiting.');
        return;
      }

      if (trimmed === 'r' || trimmed === 'refresh') {
        console.log('\nRe-extracting job listings from current page state...');
        listings = await extractJobListings(page, pattern);
        if (location) {
          const lowerFilter = location.toLowerCase();
          listings = listings.filter(
            (job) => job.location?.toLowerCase().includes(lowerFilter)
          );
          listings.forEach((job, i) => { job.index = i + 1; });
          console.log(`Filtered to ${listings.length} job(s) matching location "${location}".`);
        }
        if (listings.length > 0) {
          break;
        }
      }
    }

    // If using generic pattern, check if page content suggests a known pattern
    if (pattern.name === 'Generic') {
      const detectedPattern = await detectPatternFromContent(page);
      if (detectedPattern !== null) {
        console.log(`\nNote: Page content appears to match the '${detectedPattern}' pattern.`);
        console.log(`Re-run with: bun run job-crawler.ts "${careersUrl}" ${detectedPattern}`);
      }
    }

    console.log(`\nFound ${listings.length} job listing(s).`);

    // Handle --list flag: just display listings and exit
    if (list) {
      displayJobPage(listings, 0, listings.length);
      console.log('(--list mode: listing only, no crawling)');
      return;
    }

    // Determine selected indices based on flags
    let selectedIndices: number[];

    if (all) {
      // --all flag: select all jobs
      selectedIndices = listings.map((job) => job.index);
      console.log(`Selected all ${selectedIndices.length} job(s).`);
    } else if (first !== undefined) {
      // --first N flag: select first N jobs
      const count = Math.min(first, listings.length);
      selectedIndices = listings.slice(0, count).map((job) => job.index);
      console.log(`Selected first ${count} job(s).`);
    } else if (select !== undefined) {
      // --select "N,M,..." flag: select specific job indices
      const validIndices = select.filter(idx => listings.some(j => j.index === idx));
      selectedIndices = validIndices;
      if (validIndices.length < select.length) {
        const invalid = select.filter(idx => !validIndices.includes(idx));
        console.log(`Warning: Invalid indices ignored: ${invalid.join(', ')}`);
      }
      console.log(`Selected ${selectedIndices.length} job(s) by index.`);
    } else {
      // Interactive mode
      selectedIndices = await promptForSelection(listings, page, pattern, location);
    }

    if (selectedIndices.length === 0) {
      console.log('\nNo jobs selected. Exiting.');
      return;
    }

    // Handle --dry-run flag
    if (dryRun) {
      console.log('\n(--dry-run mode: showing what would be crawled)');
      for (const index of selectedIndices) {
        const job = listings.find((j) => j.index === index);
        if (job === undefined) continue;
        console.log(`  Would crawl: ${job.title}`);
        console.log(`    URL: ${job.url}`);
      }
      return;
    }

    const turndown = createMarkdownConverter();

    for (const index of selectedIndices) {
      const job = listings.find((j) => j.index === index);
      if (job === undefined) continue;

      try {
        await crawlJobPage(page, job, pattern, turndown, followIframe);
      } catch (error) {
        console.error(`  Error crawling ${job.title}:`, error);
        console.log('  Continuing to next job...');
      }
    }

    console.log('\nDone!');
  } catch (error) {
    console.error('Error:', error);
  } finally {
    if (page !== null) {
      await page.close();
    }
    await browser.close();
  }
})();
