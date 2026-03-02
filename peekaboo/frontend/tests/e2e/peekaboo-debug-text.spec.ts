import { test, expect } from '@playwright/test';
import { routeFixtures } from '../helpers/e2e-helpers';

test.describe('Debug text input mode', () => {
  test('text input is hidden by default', async ({ page }) => {
    await page.goto('/');

    await expect(page.locator('[data-testid="debug-text-input"]')).toBeHidden();
  });

  test('text input is visible with ?debug=text', async ({ page }) => {
    await page.goto('/?debug=text');

    await expect(page.locator('[data-testid="debug-text-input"]')).toBeVisible();
    await expect(page.locator('[data-testid="debug-text-field"]')).toBeVisible();
    await expect(page.locator('[data-testid="debug-text-submit"]')).toBeVisible();
  });

  test('submitting text command shows media', async ({ page }) => {
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

    await page.goto('/?debug=text');

    const input = page.locator('[data-testid="debug-text-field"]');
    await input.fill('show me a cat');
    await page.locator('[data-testid="debug-text-submit"]').click();

    // Input should be cleared after submit
    await expect(input).toHaveValue('');

    // Media should be displayed
    await expect(page.locator('[data-testid="media-image"]')).toBeVisible({ timeout: 5000 });
  });

  test('shows error for unknown concept', async ({ page }) => {
    await page.route('**/api/intent', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ subject: 'unicorn' }),
      })
    );

    await page.route('**/api/media/unicorn', route =>
      route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Not found' }),
      })
    );

    await page.goto('/?debug=text');

    const input = page.locator('[data-testid="debug-text-field"]');
    await input.fill('show me a unicorn');
    await page.locator('[data-testid="debug-text-submit"]').click();

    // Should show error state
    await expect(page.locator('[data-testid="media-display"]')).toContainText(/error|not found/i, { timeout: 5000 });
  });

  test('empty input does not submit', async ({ page }) => {
    const intentCalled = { value: false };
    await page.route('**/api/intent', route => {
      intentCalled.value = true;
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ subject: 'cat' }),
      });
    });

    await page.goto('/?debug=text');

    await page.locator('[data-testid="debug-text-submit"]').click();

    // Wait a bit to ensure nothing happened
    await page.waitForTimeout(500);
    expect(intentCalled.value).toBe(false);
  });
});
