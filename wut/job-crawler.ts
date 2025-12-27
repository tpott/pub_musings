import puppeteer, { type Browser, type Page, type CDPSession } from 'puppeteer';
import TurndownService from 'turndown';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';
import { createInterface } from 'readline';
import { existsSync, mkdirSync, writeFileSync } from 'fs';

const __dirname = dirname(fileURLToPath(import.meta.url));

// Type definitions
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

interface HarLog {
  version: string;
  creator: { name: string; version: string };
  entries: HarEntry[];
}

// Job board patterns for different ATS platforms
const JOB_BOARD_PATTERNS: Record<string, JobBoardPattern> = {
  greenhouse: {
    name: 'Greenhouse',
    urlPattern: /job-boards\.greenhouse\.io|boards\.greenhouse\.io/,
    selectors: [
      'a[href*="greenhouse.io/"]',
      '.opening a',
      '[data-mapped="true"] a',
    ],
    contentSelector: '#content, .content, main, article, .job-post',
  },
  lever: {
    name: 'Lever',
    urlPattern: /jobs\.lever\.co/,
    selectors: [
      'a[href*="jobs.lever.co/"]',
      '.posting-title',
      '.posting a',
    ],
    contentSelector: '.content, .posting-page, main',
  },
  workday: {
    name: 'Workday',
    urlPattern: /myworkdayjobs\.com/,
    selectors: [
      'a[href*="myworkdayjobs.com/"]',
      '[data-automation-id="jobTitle"] a',
      '.WJLV a',
    ],
    contentSelector: '[data-automation-id="jobPostingPage"], .job-posting',
  },
  ashby: {
    name: 'Ashby',
    urlPattern: /jobs\.ashbyhq\.com/,
    selectors: [
      'a[href*="jobs.ashbyhq.com/"]',
      '[data-testid="job-link"]',
      '.ashby-job-posting-brief-list a',
      '[class*="ashby"] a',
      'a[href*="/ashby"]',
      // Ashby embed job cards often use these patterns
      '[class*="JobPosting"] a',
      '[class*="job-posting"] a',
      '[class*="jobPosting"] a',
    ],
    contentSelector: '.ashby-job-posting-content, main, article',
  },
  generic: {
    name: 'Generic',
    urlPattern: /.*/,
    selectors: [
      'a[href*="/job"]',
      'a[href*="/jobs/"]',
      'a[href*="/career"]',
      'a[href*="/position"]',
      'a[href*="/opening"]',
      '.job-listing a',
      '.careers-listing a',
      '[class*="job"] a',
      '[class*="career"] a',
    ],
    contentSelector: 'main, article, .content, #content, .job-description, [class*="job"]',
  },
};

// HAR recorder using CDP
async function createHarRecorder(page: Page): Promise<{
  start: () => Promise<void>;
  stop: () => Promise<HarLog>;
}> {
  const client: CDPSession = await page.createCDPSession();
  const entries: HarEntry[] = [];
  const requestMap = new Map<string, { startTime: number; request: HarEntry['request'] }>();

  return {
    start: async () => {
      await client.send('Network.enable');

      client.on('Network.requestWillBeSent', (params) => {
        const { requestId, request, timestamp } = params;
        requestMap.set(requestId, {
          startTime: timestamp,
          request: {
            method: request.method,
            url: request.url,
            headers: Object.entries(request.headers).map(([name, value]) => ({
              name,
              value: String(value),
            })),
          },
        });
      });

      client.on('Network.responseReceived', (params) => {
        const { requestId, response, timestamp } = params;
        const requestData = requestMap.get(requestId);
        if (requestData === undefined) return;

        const entry: HarEntry = {
          startedDateTime: new Date().toISOString(),
          time: (timestamp - requestData.startTime) * 1000,
          request: requestData.request,
          response: {
            status: response.status,
            statusText: response.statusText,
            headers: Object.entries(response.headers).map(([name, value]) => ({
              name,
              value: String(value),
            })),
            content: {
              size: response.encodedDataLength || 0,
              mimeType: response.mimeType,
            },
          },
        };
        entries.push(entry);
      });
    },
    stop: async () => {
      await client.send('Network.disable');
      client.removeAllListeners();
      return {
        version: '1.2',
        creator: { name: 'job-crawler', version: '1.0.0' },
        entries,
      };
    },
  };
}

// HTML to markdown converter
function createMarkdownConverter(): TurndownService {
  const turndownService = new TurndownService({
    headingStyle: 'atx',
    codeBlockStyle: 'fenced',
  });

  // Remove script and style tags
  turndownService.remove(['script', 'style', 'nav', 'footer', 'header']);

  return turndownService;
}

// CLI argument parsing
function parseArgs(): CliArgs {
  const args = process.argv.slice(2);

  if (args.length === 0) {
    console.log('Usage: bun run job-crawler.ts <careers-url> [pattern]');
    console.log('');
    console.log('Patterns: greenhouse, lever, workday, ashby, generic (default)');
    console.log('');
    console.log('Examples:');
    console.log('  bun run job-crawler.ts "https://example.com/careers#open-positions"');
    console.log('  bun run job-crawler.ts "https://example.com/careers" lever');
    process.exit(1);
  }

  return {
    careersUrl: args[0],
    pattern: args[1],
  };
}

// Auto-detect job board pattern from URL
function detectJobBoard(url: string): JobBoardPattern {
  for (const [key, pattern] of Object.entries(JOB_BOARD_PATTERNS)) {
    if (key === 'generic') continue;
    if (pattern.urlPattern.test(url)) {
      return pattern;
    }
  }
  return JOB_BOARD_PATTERNS.generic;
}

// Detect job board pattern from page content (URLs, iframes, classes)
async function detectPatternFromContent(page: Page): Promise<string | null> {
  return page.evaluate(() => {
    // Pattern identifiers to look for in URLs and content
    const patternChecks: Array<{ name: string; urlPatterns: RegExp[]; classPatterns: RegExp[] }> = [
      {
        name: 'ashby',
        urlPatterns: [/ashbyhq\.com/, /ashby/i],
        classPatterns: [/ashby/i],
      },
      {
        name: 'greenhouse',
        urlPatterns: [/greenhouse\.io/, /boards\.greenhouse/],
        classPatterns: [/greenhouse/i],
      },
      {
        name: 'lever',
        urlPatterns: [/lever\.co/, /jobs\.lever/],
        classPatterns: [/lever/i],
      },
      {
        name: 'workday',
        urlPatterns: [/myworkdayjobs\.com/, /workday/i],
        classPatterns: [/workday/i, /WJLV/],
      },
    ];

    // Check all links on the page
    const allLinks = Array.from(document.querySelectorAll('a[href]'));
    const allUrls = allLinks.map((a) => a.getAttribute('href') || '');

    // Check iframes
    const iframes = Array.from(document.querySelectorAll('iframe'));
    const iframeUrls = iframes.map((iframe) => iframe.src || iframe.getAttribute('data-src') || '');
    allUrls.push(...iframeUrls);

    // Check scripts (some embed job boards via script)
    const scripts = Array.from(document.querySelectorAll('script[src]'));
    const scriptUrls = scripts.map((s) => s.getAttribute('src') || '');
    allUrls.push(...scriptUrls);

    // Collect all class names
    const allElements = document.querySelectorAll('[class]');
    const allClasses: string[] = [];
    allElements.forEach((el) => {
      const classes = el.getAttribute('class') || '';
      allClasses.push(classes);
    });

    for (const check of patternChecks) {
      // Check URLs
      for (const url of allUrls) {
        for (const pattern of check.urlPatterns) {
          if (pattern.test(url)) {
            return check.name;
          }
        }
      }

      // Check class names
      for (const className of allClasses) {
        for (const pattern of check.classPatterns) {
          if (pattern.test(className)) {
            return check.name;
          }
        }
      }
    }

    return null;
  });
}

// Detect job board iframes and return the iframe URL if found
async function findJobBoardIframeUrl(page: Page): Promise<string | null> {
  const iframeUrls = await page.evaluate(() => {
    const iframes = document.querySelectorAll('iframe');
    const urls: string[] = [];
    for (const iframe of iframes) {
      const src = iframe.src || iframe.getAttribute('data-src') || '';
      if (src) urls.push(src);
    }
    return urls;
  });

  const jobBoardPatterns = [
    /jobs\.ashbyhq\.com/,
    /boards\.greenhouse\.io/,
    /job-boards\.greenhouse\.io/,
    /jobs\.lever\.co/,
    /myworkdayjobs\.com/,
  ];

  for (const url of iframeUrls) {
    for (const pattern of jobBoardPatterns) {
      if (pattern.test(url)) return url;
    }
  }
  return null;
}

// Extract job listings from careers page
async function extractJobListings(page: Page, pattern: JobBoardPattern): Promise<JobListing[]> {
  const listings: JobListing[] = [];
  const seenUrls = new Set<string>();

  for (const selector of pattern.selectors) {
    try {
      const elements = await page.$$(selector);
      for (const element of elements) {
        const href = await element.evaluate((el) => el.getAttribute('href'));
        // Look for heading elements inside the anchor first (e.g., OpenAI uses <h2> for job titles)
        // Fall back to anchor's innerText if no heading found
        const text = await element.evaluate((el) => {
          const heading = el.querySelector('h1, h2, h3, h4, h5, h6');
          if (heading) {
            return (heading.textContent || '').replace(/\s+/g, ' ').trim();
          }
          return ((el as HTMLElement).innerText || '').replace(/\s+/g, ' ').trim();
        });

        if (href === null || href === '') continue;

        // Convert relative URLs to absolute
        const absoluteUrl = new URL(href, page.url()).toString();

        // Skip if already seen or is an anchor link
        if (seenUrls.has(absoluteUrl) || href.startsWith('#')) continue;
        seenUrls.add(absoluteUrl);

        // Try to extract additional info
        const parentText = await element.evaluate((el) => {
          const parent = el.closest('li, tr, div, article');
          return parent?.textContent?.trim() || '';
        });

        listings.push({
          index: listings.length + 1,
          title: text || `Job ${listings.length + 1}`,
          url: absoluteUrl,
          department: undefined,
          location: undefined,
        });
      }
    } catch {
      // Selector didn't match, continue to next
    }
  }

  return listings;
}

// Display a page of job listings
function displayJobPage(listings: JobListing[], page: number, pageSize: number): void {
  const totalPages = Math.ceil(listings.length / pageSize);
  const startIdx = page * pageSize;
  const endIdx = Math.min(startIdx + pageSize, listings.length);
  const pageJobs = listings.slice(startIdx, endIdx);

  console.log(`\n--- Jobs (Page ${page + 1}/${totalPages}) ---`);
  for (const job of pageJobs) {
    console.log(`${job.index}. ${job.title}`);
    if (job.department) console.log(`   Department: ${job.department}`);
    if (job.location) console.log(`   Location: ${job.location}`);
  }
  console.log('');
}

// Prompt user for job selection with pagination
async function promptForSelection(
  listings: JobListing[],
  page: Page,
  pattern: JobBoardPattern
): Promise<number[]> {
  const pageSize = 20;
  const totalPages = Math.ceil(listings.length / pageSize);
  let currentPage = 0;
  const selectedIndices: number[] = [];

  const rl = createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  const askQuestion = (prompt: string): Promise<string> => {
    return new Promise((resolve) => {
      rl.question(prompt, resolve);
    });
  };

  while (true) {
    displayJobPage(listings, currentPage, pageSize);

    if (selectedIndices.length > 0) {
      console.log(`Selected so far: ${selectedIndices.join(', ')}`);
    }

    const prompt = totalPages > 1
      ? 'Enter indices (e.g., "1,3,5"), "n"=next, "p"=prev, "r"=refresh, "a"=all, "d"=done, "q"=quit: '
      : 'Enter indices (e.g., "1,3,5"), "r"=refresh, "a"=all, "d"=done, "q"=quit: ';

    const answer = (await askQuestion(prompt)).toLowerCase().trim();

    if (answer === 'q' || answer === 'quit') {
      rl.close();
      return [];
    }

    if (answer === 'd' || answer === 'done') {
      rl.close();
      return selectedIndices;
    }

    if (answer === 'n' || answer === 'next') {
      if (currentPage < totalPages - 1) {
        currentPage++;
      } else {
        console.log('Already on last page.');
      }
      continue;
    }

    if (answer === 'p' || answer === 'prev') {
      if (currentPage > 0) {
        currentPage--;
      } else {
        console.log('Already on first page.');
      }
      continue;
    }

    if (answer === 'r' || answer === 'refresh') {
      console.log('\nRe-extracting job listings from current page state...');
      const newListings = await extractJobListings(page, pattern);
      if (newListings.length === 0) {
        console.log('No jobs found. Try scrolling or adjusting filters.');
        continue;
      }
      // Reset state with new listings
      listings.length = 0;
      listings.push(...newListings);
      selectedIndices.length = 0;
      currentPage = 0;
      console.log(`Found ${listings.length} job listing(s).`);
      continue;
    }

    if (answer === 'a' || answer === 'all') {
      rl.close();
      return listings.map((job) => job.index);
    }

    // Parse indices
    const indices = answer
      .split(',')
      .map((s) => parseInt(s.trim(), 10))
      .filter((n) => !isNaN(n) && n >= 1 && n <= listings.length);

    if (indices.length === 0) continue;

    for (const idx of indices) {
      if (!selectedIndices.includes(idx)) {
        selectedIndices.push(idx);
      }
    }
    console.log(`Added: ${indices.join(', ')}`);

    // If single page, return immediately after selection
    if (totalPages === 1) {
      rl.close();
      return selectedIndices;
    }

    // Advance to next page after selection
    if (currentPage < totalPages - 1) {
      currentPage++;
    }
  }
}

// Generate safe filename from job title
function generateFilename(title: string): string {
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
  const safeTitle = title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 50);
  return `${safeTitle}-${timestamp}`;
}

// Ensure output directories exist
function ensureDirectories(): void {
  const dataDir = join(__dirname, 'data');
  const harDir = join(dataDir, 'har');
  const jobsDir = join(dataDir, 'jobs');

  if (!existsSync(dataDir)) mkdirSync(dataDir);
  if (!existsSync(harDir)) mkdirSync(harDir);
  if (!existsSync(jobsDir)) mkdirSync(jobsDir);
}

// Crawl a single job page
async function crawlJobPage(
  page: Page,
  job: JobListing,
  pattern: JobBoardPattern,
  turndown: TurndownService
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

  const harLog = await harRecorder.stop();

  // Save HAR file
  const filename = generateFilename(job.title);
  const domain = new URL(job.url).hostname;
  const harDir = join(__dirname, 'data', 'har', domain);
  if (!existsSync(harDir)) mkdirSync(harDir, { recursive: true });
  const harPath = join(harDir, `${filename}.har`);
  writeFileSync(harPath, JSON.stringify({ log: harLog }, null, 2));
  console.log(`  HAR saved: ${harPath}`);

  // Remove application forms and job alert sections before extracting content
  await page.evaluate(() => {
    // Selectors for elements to remove
    const selectorsToRemove = [
      'form',
      '[id*="application"]',
      '[class*="application"]',
      '[id*="apply"]',
      '[class*="apply-form"]',
      '[data-qa*="apply"]',
    ];

    for (const selector of selectorsToRemove) {
      document.querySelectorAll(selector).forEach((el) => el.remove());
    }

    // Remove elements containing "Apply for this job" or "Create a Job Alert" text
    const textPatternsToRemove = [
      /apply for this job/i,
      /create a job alert/i,
      /submit.*application/i,
    ];

    const allElements = document.querySelectorAll('h1, h2, h3, h4, h5, h6, div, section');
    for (const el of allElements) {
      const text = el.textContent || '';
      const matchedPattern = textPatternsToRemove.find((p) => p.test(text));
      if (!matchedPattern) continue;
      if (!el.parentNode) continue;

      // Check if this is a container with form-like content
      const hasFormElements = el.querySelector('input, select, textarea, button[type="submit"]');
      if (hasFormElements !== null || el.querySelectorAll('label').length > 3) {
        el.remove();
        continue;
      }

      // For non-headings, skip
      if (!/^H[1-6]$/.test(el.tagName)) continue;

      // For headings, remove the section (find parent section/div)
      const parent = el.closest('section, div.section, [class*="section"]');
      if (parent !== null && parent.querySelector('input, select, textarea')) {
        parent.remove();
        continue;
      }

      // Just remove everything after this heading in its parent
      let sibling = el.nextElementSibling;
      while (sibling !== null) {
        const next = sibling.nextElementSibling;
        sibling.remove();
        sibling = next;
      }
      el.remove();
    }
  });

  // Extract content and convert to markdown
  let html = '';
  try {
    const contentElement = await page.$(pattern.contentSelector);
    if (contentElement !== null) {
      html = await contentElement.evaluate((el) => el.innerHTML);
    } else {
      // Fallback to body
      html = await page.evaluate(() => document.body.innerHTML);
    }
  } catch {
    html = await page.evaluate(() => document.body.innerHTML);
  }

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
  const { careersUrl, pattern: patternName } = parseArgs();

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
  ensureDirectories();

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
    const iframeUrl = await findJobBoardIframeUrl(page);
    if (iframeUrl !== null) {
      console.log(`\nDetected job board iframe, redirecting to: ${iframeUrl}`);
      await page.goto(iframeUrl, { waitUntil: 'networkidle2', timeout: 60000 });
      pattern = detectJobBoard(iframeUrl);
      console.log(`Using pattern for iframe: ${pattern.name}`);
    }

    console.log('Extracting job listings...');
    let listings = await extractJobListings(page, pattern);

    // If no listings found, allow user to navigate and retry
    while (listings.length === 0) {
      console.log('\nNo job listings found. Try a different pattern or check the URL.');
      console.log('Available patterns: greenhouse, lever, workday, ashby, generic');
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

    const selectedIndices = await promptForSelection(listings, page, pattern);

    if (selectedIndices.length === 0) {
      console.log('\nNo jobs selected. Exiting.');
      return;
    }

    const turndown = createMarkdownConverter();

    for (const index of selectedIndices) {
      const job = listings.find((j) => j.index === index);
      if (job === undefined) continue;

      try {
        await crawlJobPage(page, job, pattern, turndown);
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
