import type { Page } from 'puppeteer';
import type { JobBoardPattern } from './types.js';

// Job board patterns for different ATS platforms
export const JOB_BOARD_PATTERNS: Record<string, JobBoardPattern> = {
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
  'greenhouse-embed': {
    name: 'Greenhouse Embed',
    urlPattern: /gh_jid=/,  // Detect pages with Greenhouse job IDs
    selectors: [
      'a[data-link*="gh_jid="]',     // SoFi-style: data-link attribute
      '.listing a[data-link]',       // Jobs in .listing containers
      'a[href*="gh_jid="]',          // Standard href with gh_jid
      'a[href*="/careers/job/"]',    // Common careers path pattern
    ],
    contentSelector: '[jd-content], .job-description, main, article, .content, #content',
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

// Auto-detect job board pattern from URL
export function detectJobBoard(url: string): JobBoardPattern {
  for (const [key, pattern] of Object.entries(JOB_BOARD_PATTERNS)) {
    if (key === 'generic') continue;
    if (pattern.urlPattern.test(url)) {
      return pattern;
    }
  }
  return JOB_BOARD_PATTERNS.generic;
}

// Detect job board pattern from page content (URLs, iframes, classes)
export async function detectPatternFromContent(page: Page): Promise<string | null> {
  return page.evaluate(() => {
    // Pattern identifiers to look for in URLs and content
    const patternChecks: Array<{ name: string; urlPatterns: RegExp[]; classPatterns: RegExp[] }> = [
      {
        name: 'ashby',
        urlPatterns: [/ashbyhq\.com/, /ashby/i],
        classPatterns: [/ashby/i],
      },
      {
        name: 'greenhouse-embed',
        urlPatterns: [/gh_jid=/, /\/careers\/job\//],
        classPatterns: [],
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
