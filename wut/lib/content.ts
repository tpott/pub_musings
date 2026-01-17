import type { Page } from 'puppeteer';
import type { JobBoardPattern } from './types.js';

// Clean DOM by removing application forms and job alert sections
export async function cleanDom(page: Page): Promise<void> {
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
}

// Extract HTML content from the page using the pattern's content selector
// Tries each selector individually and picks the one with the most content
export async function extractContent(page: Page, pattern: JobBoardPattern): Promise<string> {
  const selectors = pattern.contentSelector.split(',').map((s) => s.trim());

  try {
    // Try each selector and find the one with the most text content
    const result = await page.evaluate((sels: string[]) => {
      let bestHtml = '';
      let bestLength = 0;

      for (const selector of sels) {
        const el = document.querySelector(selector);
        if (!el) continue;

        const text = el.textContent || '';
        // Prefer elements with more text content
        if (text.length > bestLength) {
          bestLength = text.length;
          bestHtml = el.innerHTML;
        }
      }

      // Fallback to body if nothing found
      if (!bestHtml) {
        bestHtml = document.body.innerHTML;
      }

      return bestHtml;
    }, selectors);

    return result;
  } catch {
    return page.evaluate(() => document.body.innerHTML);
  }
}
