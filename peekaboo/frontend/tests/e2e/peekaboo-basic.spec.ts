import { test, expect } from '@playwright/test';
import {
  getSimpleMockScript,
  getPermissionDeniedMockScript,
} from '../helpers/mock-media-recorder';
import { setupHttpMocks, routeFixtures } from '../helpers/e2e-helpers';

test.describe('Peekaboo voice command flow', () => {
  test('voice command shows cat media', async ({ page }) => {
    await page.addInitScript(getSimpleMockScript());

    await page.route('**/api/transcribe', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ text: 'show me a cat' }),
      })
    );

    await page.route('**/api/intent', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ subject: 'cat' }),
      })
    );

    await page.route('**/api/media/cat', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          photo_url: '/fixtures/mock-cat-photo.jpg',
          audio_url: '/fixtures/mock-cat-audio.mp3',
        }),
      })
    );

    await routeFixtures(page);

    await page.goto('/?useWebSocket=false');

    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();
    await expect(page.locator('[data-testid="media-display"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 10000 });

    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');

    const audio = page.locator('[data-testid="media-audio"]');
    await expect(audio).toHaveAttribute('src', '/fixtures/mock-cat-audio.mp3');
  });

  test('displays error state on API failure', async ({ page }) => {
    await page.addInitScript(getSimpleMockScript());

    await page.route('**/api/transcribe', route =>
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Transcription failed' }),
      })
    );

    await page.goto('/?useWebSocket=false');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

    await page.waitForTimeout(1000);

    await expect(page.locator('[data-testid="media-image"]')).not.toBeVisible();
  });

  test('voice command shows dog media', async ({ page }) => {
    await page.addInitScript(getSimpleMockScript());
    await setupHttpMocks(page, 'dog', 'show me a dog');

    await page.goto('/?useWebSocket=false');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();
    await expect(page.locator('[data-testid="media-display"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 10000 });

    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toHaveAttribute('src', '/fixtures/mock-dog-photo.jpg');

    const audio = page.locator('[data-testid="media-audio"]');
    await expect(audio).toHaveAttribute('src', '/fixtures/mock-dog-audio.mp3');
  });

  test('voice command shows duck media', async ({ page }) => {
    await page.addInitScript(getSimpleMockScript());
    await setupHttpMocks(page, 'duck', 'I want to see a duck');

    await page.goto('/?useWebSocket=false');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();
    await expect(page.locator('[data-testid="media-display"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 10000 });

    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toHaveAttribute('src', '/fixtures/mock-duck-photo.jpg');

    const audio = page.locator('[data-testid="media-audio"]');
    await expect(audio).toHaveAttribute('src', '/fixtures/mock-duck-audio.mp3');
  });

  test('error recovery: transcribe succeeds but intent fails, then retry succeeds', async ({ page }) => {
    let intentCallCount = 0;

    await page.addInitScript(getSimpleMockScript());

    await page.route('**/api/transcribe', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ text: 'show me a cat' }),
      })
    );

    // First attempt fails with 400 (not retried by fetchWithRetry)
    await page.route('**/api/intent', route => {
      intentCallCount++;
      if (intentCallCount === 1) {
        route.fulfill({
          status: 400,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'invalid request' }),
        });
      } else {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ subject: 'cat' }),
        });
      }
    });

    await page.route('**/api/media/cat', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          photo_url: '/fixtures/mock-cat-photo.jpg',
          audio_url: '/fixtures/mock-cat-audio.mp3',
        }),
      })
    );

    await routeFixtures(page);

    await page.goto('/?useWebSocket=false');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const mediaDisplay = page.locator('[data-testid="media-display"]');

    // First attempt - should fail on intent
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

    await page.waitForTimeout(1000);

    await expect(mediaDisplay).toContainText(/invalid request|something went wrong|try again/i);
    await expect(micButton).toHaveAttribute('aria-label', /try again/i);
    await expect(page.locator('[data-testid="media-image"]')).not.toBeVisible();

    // Wait for automatic reset to idle state (3 seconds)
    await page.waitForTimeout(3500);

    // Retry - should succeed
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 10000 });

    const img = page.locator('[data-testid="media-image"]');
    await expect(img).toHaveAttribute('src', '/fixtures/mock-cat-photo.jpg');
  });

  test('TTS synthesis is called after displaying media', async ({ page }) => {
    let ttsWasCalled = false;
    let ttsText = '';

    await page.addInitScript(getSimpleMockScript());

    await page.route('**/api/transcribe', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ text: 'show me a cat' }),
      })
    );

    await page.route('**/api/intent', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ subject: 'cat' }),
      })
    );

    await page.route('**/api/media/cat', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          photo_url: '/fixtures/mock-cat-photo.jpg',
          audio_url: '/fixtures/mock-cat-audio.mp3',
        }),
      })
    );

    // Mock TTS speak endpoint
    await page.route('**/api/speak', async route => {
      const request = route.request();
      const postData = request.postDataJSON();
      ttsWasCalled = true;
      ttsText = postData?.text || '';

      const wavHeader = new Uint8Array([
        0x52, 0x49, 0x46, 0x46, 0x24, 0x00, 0x00, 0x00,
        0x57, 0x41, 0x56, 0x45, 0x66, 0x6d, 0x74, 0x20,
        0x10, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00,
        0x44, 0xac, 0x00, 0x00, 0x88, 0x58, 0x01, 0x00,
        0x02, 0x00, 0x10, 0x00, 0x64, 0x61, 0x74, 0x61,
        0x00, 0x00, 0x00, 0x00,
      ]);

      route.fulfill({
        status: 200,
        contentType: 'audio/wav',
        body: Buffer.from(wavHeader),
      });
    });

    await routeFixtures(page);

    await page.goto('/?useWebSocket=false');
    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    await micButton.click();
    await page.waitForTimeout(200);
    await micButton.click();

    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 10000 });

    // Wait for TTS to be called (async after media display)
    await page.waitForTimeout(500);

    expect(ttsWasCalled).toBe(true);
    expect(ttsText).toBe('Here is a cat!');
  });
});

test.describe('Microphone permission handling', () => {
  test('displays error message when microphone permission is denied', async ({ page }) => {
    await page.addInitScript(getPermissionDeniedMockScript());

    await page.goto('/?useWebSocket=false');

    await expect(page.locator('[data-testid="mic-button"]')).toBeVisible();
    await expect(page.locator('[data-testid="media-display"]')).toBeVisible();

    const micButton = page.locator('[data-testid="mic-button"]');
    const mediaDisplay = page.locator('[data-testid="media-display"]');

    await micButton.click();

    await page.waitForTimeout(500);

    await expect(mediaDisplay).toContainText(/permission|denied|access|microphone|try again/i);
    await expect(micButton).toHaveAttribute('aria-label', /try again/i);
    await expect(page.locator('[data-testid="media-image"]')).not.toBeVisible();
  });
});
