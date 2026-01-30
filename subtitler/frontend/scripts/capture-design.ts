/**
 * Design Capture Script
 *
 * Uses Playwright to capture screenshots and HAR files from design reference sites.
 * Run with: cd frontend && node --import tsx scripts/capture-design.ts
 */

import { chromium, Browser, Page, BrowserContext } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const OUTPUT_DIR = path.join(__dirname, '..', '..', 'design-analysis');

interface SiteConfig {
  name: string;
  url: string;
  pages: string[];
}

const SITES: SiteConfig[] = [
  {
    name: 'anthropic',
    url: 'https://www.anthropic.com',
    pages: ['/', '/claude', '/research', '/company'],
  },
  {
    name: 'ampcode',
    url: 'https://ampcode.com',
    pages: ['/'],
  },
];

async function ensureDir(dir: string): Promise<void> {
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
}

async function captureSite(
  browser: Browser,
  site: SiteConfig
): Promise<void> {
  const siteDir = path.join(OUTPUT_DIR, site.name);
  await ensureDir(siteDir);
  await ensureDir(path.join(siteDir, 'screenshots'));
  await ensureDir(path.join(siteDir, 'har'));

  console.log(`\n📸 Capturing ${site.name}...`);

  for (const pagePath of site.pages) {
    const pageUrl = site.url + pagePath;
    const safeName = pagePath === '/' ? 'home' : pagePath.replace(/\//g, '-').slice(1);

    console.log(`  → ${pageUrl}`);

    // Create a new context with HAR recording for each page
    const context: BrowserContext = await browser.newContext({
      recordHar: {
        path: path.join(siteDir, 'har', `${safeName}.har`),
        mode: 'full',
      },
      viewport: { width: 1920, height: 1080 },
    });

    const page: Page = await context.newPage();

    try {
      await page.goto(pageUrl, { waitUntil: 'networkidle', timeout: 30000 });

      // Wait a bit for any animations to settle
      await page.waitForTimeout(1000);

      // Capture full-page screenshot
      await page.screenshot({
        path: path.join(siteDir, 'screenshots', `${safeName}-full.png`),
        fullPage: true,
      });

      // Capture above-the-fold screenshot
      await page.screenshot({
        path: path.join(siteDir, 'screenshots', `${safeName}-viewport.png`),
        fullPage: false,
      });

      // Capture mobile viewport
      await page.setViewportSize({ width: 375, height: 812 });
      await page.waitForTimeout(500);
      await page.screenshot({
        path: path.join(siteDir, 'screenshots', `${safeName}-mobile.png`),
        fullPage: false,
      });

      // Extract page metadata
      const metadata = await page.evaluate(() => {
        const getComputedColors = () => {
          const body = document.body;
          const computed = window.getComputedStyle(body);
          return {
            background: computed.backgroundColor,
            color: computed.color,
            fontFamily: computed.fontFamily,
          };
        };

        const getHeadings = () => {
          const headings: { level: string; text: string }[] = [];
          document.querySelectorAll('h1, h2, h3').forEach((h) => {
            headings.push({
              level: h.tagName.toLowerCase(),
              text: (h.textContent || '').trim().substring(0, 100),
            });
          });
          return headings.slice(0, 10);
        };

        const getNavLinks = () => {
          const links: string[] = [];
          document.querySelectorAll('nav a, header a').forEach((a) => {
            const text = (a.textContent || '').trim();
            if (text && text.length < 50) {
              links.push(text);
            }
          });
          return [...new Set(links)].slice(0, 15);
        };

        const getCTAs = () => {
          const ctas: string[] = [];
          document.querySelectorAll('a, button').forEach((el) => {
            const text = (el.textContent || '').trim();
            const classes = el.className || '';
            if (
              text.length > 0 &&
              text.length < 30 &&
              (classes.includes('btn') ||
                classes.includes('cta') ||
                classes.includes('button') ||
                el.tagName === 'BUTTON' ||
                (el as HTMLElement).style.backgroundColor)
            ) {
              ctas.push(text);
            }
          });
          return [...new Set(ctas)].slice(0, 10);
        };

        return {
          title: document.title,
          description:
            document
              .querySelector('meta[name="description"]')
              ?.getAttribute('content') || '',
          colors: getComputedColors(),
          headings: getHeadings(),
          navLinks: getNavLinks(),
          ctas: getCTAs(),
        };
      });

      // Save metadata
      fs.writeFileSync(
        path.join(siteDir, `${safeName}-metadata.json`),
        JSON.stringify(metadata, null, 2)
      );

      console.log(`    ✓ Captured: ${metadata.title}`);
    } catch (error) {
      console.error(`    ✗ Error capturing ${pageUrl}:`, error);
    } finally {
      await context.close();
    }
  }
}

async function main(): Promise<void> {
  console.log('🎨 Design Template Analysis - Capture Script');
  console.log('=' .repeat(50));

  await ensureDir(OUTPUT_DIR);

  const browser = await chromium.launch({
    headless: true,
  });

  try {
    for (const site of SITES) {
      await captureSite(browser, site);
    }
  } finally {
    await browser.close();
  }

  console.log('\n✅ Capture complete!');
  console.log(`📁 Output directory: ${OUTPUT_DIR}`);
}

main().catch(console.error);
