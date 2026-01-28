// Temporary research script: Dump DOM structure of job listing elements
// Usage: bun run study-page.ts "https://microsoft.ai/careers/"
// Delete after confirming selectors work for location extraction.

import puppeteer from 'puppeteer';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

const url = process.argv[2];
if (!url) {
  console.log('Usage: bun run study-page.ts <careers-url>');
  process.exit(1);
}

(async () => {
  const userDataDir = join(__dirname, 'chrome-user-data');
  const browser = await puppeteer.launch({
    headless: false,
    userDataDir,
    args: [`--user-data-dir=${userDataDir}`, '--no-sandbox', '--disable-setuid-sandbox'],
  });

  const page = await browser.newPage();
  await page.setViewport({ width: 1280, height: 800 });

  console.log(`Navigating to: ${url}`);
  await page.goto(url, { waitUntil: 'networkidle2', timeout: 60000 });

  // Wait a moment for any client-side rendering
  await new Promise((r) => setTimeout(r, 3000));

  // Dump first 5 job-like link elements
  const selectors = [
    'a[href*="/job"]',
    'a[href*="/career"]',
    'a[href*="/position"]',
    'a[href*="/opening"]',
    'a[href*="/roles/"]',
  ];

  for (const selector of selectors) {
    const elements = await page.$$(selector);
    if (elements.length === 0) continue;

    console.log(`\n=== Selector: ${selector} (${elements.length} matches) ===\n`);

    const sample = elements.slice(0, 5);
    for (let i = 0; i < sample.length; i++) {
      const info = await sample[i].evaluate((el) => {
        const htmlEl = el as HTMLElement;
        const style = window.getComputedStyle(htmlEl);
        const visible = style.display !== 'none' && style.visibility !== 'hidden'
          && (htmlEl.offsetWidth > 0 || htmlEl.offsetHeight > 0);

        // Find parent container
        const parent = el.closest('li, tr, div, article, [class*="card"], [class*="listing"]');
        const parentTag = parent ? `<${parent.tagName.toLowerCase()} class="${parent.className}">` : 'none';
        const parentHTML = parent ? (parent as HTMLElement).outerHTML.substring(0, 1500) : 'N/A';

        // Look for location-like elements in parent
        let locationText = '';
        if (parent) {
          const locEl = parent.querySelector(
            '[class*="location"], [class*="Location"], [class*="region"], [data-field="location"]'
          );
          if (locEl) {
            locationText = `[SELECTOR HIT] ${locEl.className}: "${locEl.textContent?.trim()}"`;
          }
        }

        return {
          href: el.getAttribute('href'),
          text: htmlEl.innerText?.replace(/\s+/g, ' ').trim().substring(0, 200),
          visible,
          parentTag,
          parentHTML,
          locationText,
        };
      });

      console.log(`--- Element ${i + 1} ---`);
      console.log(`  href: ${info.href}`);
      console.log(`  text: ${info.text}`);
      console.log(`  visible: ${info.visible}`);
      console.log(`  parent: ${info.parentTag}`);
      if (info.locationText) {
        console.log(`  location: ${info.locationText}`);
      }
      console.log(`  parentHTML (first 1500 chars):`);
      console.log(info.parentHTML);
      console.log('');
    }
  }

  console.log('\nDone studying page. Close the browser manually or press Ctrl+C.');
  await browser.close();
})();
