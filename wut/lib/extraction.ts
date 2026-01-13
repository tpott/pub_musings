import type { Page } from 'puppeteer';
import type { JobBoardPattern, JobListing } from './types.js';

// Extract job listings from careers page
// This is the primary file to debug when job titles are wrong (e.g., "Job 156" instead of real titles)
export async function extractJobListings(page: Page, pattern: JobBoardPattern): Promise<JobListing[]> {
  const listings: JobListing[] = [];
  const seenUrls = new Set<string>();

  for (const selector of pattern.selectors) {
    try {
      const elements = await page.$$(selector);
      for (const element of elements) {
        const href = await element.evaluate((el) =>
          el.getAttribute('href') || el.getAttribute('data-link')
        );
        // Look for job-title class first (SoFi pattern), then heading elements, then innerText
        const text = await element.evaluate((el) => {
          // Check for job-title class first (SoFi pattern)
          const jobTitle = el.querySelector('.job-title');
          if (jobTitle) return (jobTitle.textContent || '').replace(/\s+/g, ' ').trim();

          // Fall back to heading tags (e.g., OpenAI uses <h2> for job titles)
          const heading = el.querySelector('h1, h2, h3, h4, h5, h6');
          if (heading) return (heading.textContent || '').replace(/\s+/g, ' ').trim();

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
