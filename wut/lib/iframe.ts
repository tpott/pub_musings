import type { Page } from 'puppeteer';

// Detect job board iframes and return the iframe URL if found
export async function findJobBoardIframeUrl(page: Page): Promise<string | null> {
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
