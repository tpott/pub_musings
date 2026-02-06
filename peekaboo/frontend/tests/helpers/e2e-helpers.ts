import { Page } from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export const fixturesDir = path.join(__dirname, '..', '..', '..', 'tests', 'fixtures');

/**
 * Route fixture files from the test fixtures directory.
 */
export async function routeFixtures(page: Page): Promise<void> {
  await page.route('**/fixtures/**', async route => {
    const url = new URL(route.request().url());
    const filePath = path.join(fixturesDir, url.pathname.replace('/fixtures/', ''));
    await route.fulfill({ path: filePath });
  });
}

/**
 * Set up mocks for a complete HTTP-mode voice command flow (transcribe + intent + media + fixtures).
 */
export async function setupHttpMocks(
  page: Page,
  animal: string,
  phrase: string
): Promise<void> {
  await page.route('**/api/transcribe', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ text: phrase }),
    })
  );

  await page.route('**/api/intent', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ subject: animal }),
    })
  );

  await page.route(`**/api/media/${animal}`, route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        photo_url: `/fixtures/mock-${animal}-photo.jpg`,
        audio_url: `/fixtures/mock-${animal}-audio.mp3`,
      }),
    })
  );

  await routeFixtures(page);
}
