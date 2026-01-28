import type { Page } from 'puppeteer';
import type { JobBoardPattern, JobListing } from './types.js';

interface ExtractedElement {
  href: string;
  text: string;
  location: string | null;
}

// Extract job listings from careers page
// This is the primary file to debug when job titles are wrong (e.g., "Job 156" instead of real titles)
export async function extractJobListings(page: Page, pattern: JobBoardPattern): Promise<JobListing[]> {
  const listings: JobListing[] = [];
  const seenUrls = new Set<string>();

  for (const selector of pattern.selectors) {
    try {
      const elements = await page.$$(selector);
      for (const element of elements) {
        // Single consolidated evaluate call: visibility check + href + text + location
        const result = await element.evaluate((el): ExtractedElement | null => {
          const htmlEl = el as HTMLElement;

          // Container selector — includes custom elements like scroll-object (microsoft.ai)
          const containerSelector = 'li, tr, article, scroll-object, [class*="card"], [class*="listing"], [class*="job"], [data-slide]';

          // Visibility check: skip hidden elements
          const style = window.getComputedStyle(htmlEl);
          if (style.display === 'none' || style.visibility === 'hidden') return null;
          if (htmlEl.offsetWidth === 0 && htmlEl.offsetHeight === 0) return null;

          // Also check the parent container visibility
          const container = htmlEl.closest(containerSelector);
          if (container) {
            const containerStyle = window.getComputedStyle(container);
            if (containerStyle.display === 'none' || containerStyle.visibility === 'hidden') return null;
            if ((container as HTMLElement).offsetWidth === 0 && (container as HTMLElement).offsetHeight === 0) return null;
          }

          // Extract href
          const href = el.getAttribute('href') || el.getAttribute('data-link');
          if (!href) return null;

          // Extract title text
          // Check for job-title class first (SoFi pattern)
          let text = '';
          const jobTitle = el.querySelector('.job-title');
          if (jobTitle) {
            text = (jobTitle.textContent || '').replace(/\s+/g, ' ').trim();
          } else {
            // Fall back to heading tags (e.g., OpenAI uses <h2> for job titles)
            const heading = el.querySelector('h1, h2, h3, h4, h5, h6');
            if (heading) {
              text = (heading.textContent || '').replace(/\s+/g, ' ').trim();
            } else {
              text = (htmlEl.innerText || '').replace(/\s+/g, ' ').trim();
            }
          }

          // Extract location
          let location: string | null = null;
          const parent = htmlEl.closest(containerSelector);

          if (parent) {
            // Strategy 1a: Selector-based — known location classes/attributes
            const locationEl = parent.querySelector(
              '[class*="location"], [class*="Location"], [class*="region"], [data-field="location"]'
            );
            if (locationEl) {
              location = (locationEl.textContent || '').replace(/\s+/g, ' ').trim();
            }

            // Strategy 1b: Metadata spans (e.g., microsoft.ai uses .text-meta for location)
            if (!location) {
              const metaEl = parent.querySelector('.text-meta, [class*="meta-location"], [class*="job-location"]');
              if (metaEl) {
                location = (metaEl.textContent || '').replace(/\s+/g, ' ').trim();
              }
            }

            // Strategy 1c: data-regions attribute on the container (e.g., microsoft.ai scroll-object)
            if (!location) {
              const regions = parent.getAttribute('data-regions');
              if (regions) {
                // Convert "redmond-united-states" → "Redmond United States"
                location = regions.split('-').map(
                  (w: string) => w.charAt(0).toUpperCase() + w.slice(1)
                ).join(' ');
              }
            }

            // Strategy 2: Heuristic fallback — strip title from parent text, look for "City, State/Country"
            if (!location) {
              let parentText = (parent.textContent || '').replace(/\s+/g, ' ').trim();
              // Remove the title text to isolate metadata
              if (text) {
                parentText = parentText.replace(text, '').trim();
              }
              const cityMatch = parentText.match(/([A-Z][a-zA-Z\s]+,\s*[A-Z][a-zA-Z\s]*)/);
              if (cityMatch) {
                location = cityMatch[1].trim();
              }
            }
          }

          return { href, text, location };
        });

        if (result === null) continue;

        const { href, text, location } = result;

        // Convert relative URLs to absolute
        const absoluteUrl = new URL(href, page.url()).toString();

        // Skip if already seen or is an anchor link
        if (seenUrls.has(absoluteUrl) || href.startsWith('#')) continue;
        seenUrls.add(absoluteUrl);

        listings.push({
          index: listings.length + 1,
          title: text || `Job ${listings.length + 1}`,
          url: absoluteUrl,
          department: undefined,
          location: location || undefined,
        });
      }
    } catch {
      // Selector didn't match, continue to next
    }
  }

  return listings;
}
